package pin

import (
	"context"
	"log"
	"time"

	"github.com/besoeasy/originless/internal/config"
	"github.com/besoeasy/originless/internal/ipfs"
	"github.com/besoeasy/originless/internal/store"
)

type Manager struct {
	db        *store.Store
	ipfs      *ipfs.Client
	limit     int64
	threshold float64
	expiry    time.Duration
}

func New(db *store.Store, ipfsClient *ipfs.Client, limit int64) *Manager {
	return &Manager{
		db:        db,
		ipfs:      ipfsClient,
		limit:     limit,
		threshold: float64(config.PinThreshold) / 100.0,
		expiry:    time.Duration(config.PinExpiryDays) * 24 * time.Hour,
	}
}

func (m *Manager) PinOnUpload(cid, filename string, size int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.ipfs.PinAdd(ctx, cid); err != nil {
		log.Printf("[pin] failed to pin %s: %v", cid, err)
		return err
	}

	if err := m.db.InsertUpload(cid, filename, size); err != nil {
		log.Printf("[pin] db insert failed for %s: %v — rolling back pin", cid, err)
		if rbErr := m.ipfs.PinRemove(context.Background(), cid); rbErr != nil {
			log.Printf("[pin] rollback unpin failed for %s: %v", cid, rbErr)
		}
		return err
	}

	pinnedSize, _ := m.db.GetPinnedSize()
	log.Printf("[pin] pinned %s (%s) — total pinned: %s",
		cid, config.FormatBytes(size), config.FormatBytes(pinnedSize))

	if float64(pinnedSize) > float64(m.limit)*m.threshold {
		log.Printf("[pin] storage exceeds %d%% threshold, evicting oldest", config.PinThreshold)
		if err := m.EvictOldest(); err != nil {
			log.Printf("[pin] eviction failed: %v", err)
		}
	}

	return nil
}

func (m *Manager) Reconcile() error {
	log.Printf("[pin] starting startup reconciliation...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ipfsPins, err := m.ipfs.PinList(ctx)
	if err != nil {
		log.Printf("[pin] failed to list IPFS pins: %v (skipping reconciliation)", err)
		return err
	}

	dbCIDs, err := m.db.GetAllTrackedCIDs()
	if err != nil {
		log.Printf("[pin] failed to get tracked CIDs: %v", err)
		return err
	}

	var orphaned int
	for cid := range ipfsPins {
		if _, tracked := dbCIDs[cid]; !tracked {
			size, err := m.ipfs.ObjectStat(ctx, cid)
			if err != nil {
				log.Printf("[pin] failed to stat orphan %s: %v (using size=0)", cid, err)
				size = 0
			}
			if err := m.db.InsertOrphaned(cid, size); err != nil {
				log.Printf("[pin] failed to import orphan %s: %v", cid, err)
				continue
			}
			orphaned++
		}
	}

	var missing []string
	for cid, unpinned := range dbCIDs {
		if !unpinned {
			if _, exists := ipfsPins[cid]; !exists {
				missing = append(missing, cid)
			}
		}
	}

	if len(missing) > 0 {
		if err := m.db.MarkMissingAsUnpinned(missing); err != nil {
			log.Printf("[pin] failed to mark missing: %v", err)
		}
	}

	pinnedCount, _ := m.db.GetPinnedCount()
	pinnedSize, _ := m.db.GetPinnedSize()
	log.Printf("[pin] reconciliation done: %d orphaned imported, %d missing marked — %d pinned (%s)",
		orphaned, len(missing), pinnedCount, config.FormatBytes(pinnedSize))

	return nil
}

func (m *Manager) Run(ctx context.Context, interval time.Duration) {
	log.Printf("[pin] janitor started (interval: %s, expiry: %s, threshold: %d%%)",
		interval, m.expiry, config.PinThreshold)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[pin] janitor stopped")
			return
		case <-ticker.C:
			if err := m.UnpinExpired(); err != nil {
				log.Printf("[pin] expired unpin cycle error: %v", err)
			}
			if err := m.CheckThreshold(); err != nil {
				log.Printf("[pin] threshold check error: %v", err)
			}
		}
	}
}

func (m *Manager) UnpinExpired() error {
	expired, err := m.db.GetExpiredUnpins(m.expiry)
	if err != nil {
		return err
	}

	if len(expired) == 0 {
		return nil
	}

	log.Printf("[pin] unpinning %d expired files", len(expired))

	var unpinned int
	for _, u := range expired {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := m.ipfs.PinRemove(ctx, u.CID); err != nil {
			cancel()
			log.Printf("[pin] failed to unpin %s: %v", u.CID, err)
			continue
		}
		cancel()
		if err := m.db.MarkUnpinned(u.CID); err != nil {
			log.Printf("[pin] failed to mark unpinned %s: %v", u.CID, err)
			continue
		}
		unpinned++
	}

	pinnedSize, _ := m.db.GetPinnedSize()
	log.Printf("[pin] unpinned %d expired files — pinned: %s", unpinned, config.FormatBytes(pinnedSize))
	return nil
}

func (m *Manager) CheckThreshold() error {
	pinnedSize, err := m.db.GetPinnedSize()
	if err != nil {
		return err
	}

	if float64(pinnedSize) <= float64(m.limit)*m.threshold {
		return nil
	}

	log.Printf("[pin] pinned size %s exceeds %d%% of %s, evicting",
		config.FormatBytes(pinnedSize), config.PinThreshold, config.FormatBytes(m.limit))

	return m.EvictOldest()
}

func (m *Manager) EvictOldest() error {
	pinnedSize, err := m.db.GetPinnedSize()
	if err != nil {
		return err
	}

	targetSize := int64(float64(m.limit) * m.threshold)
	if pinnedSize <= targetSize {
		return nil
	}

	var freed int64
	var unpinned int

	for pinnedSize > targetSize {
		oldest, err := m.db.GetOldestPinnedFiles(50)
		if err != nil {
			return err
		}
		if len(oldest) == 0 {
			log.Printf("[pin] warning: cannot evict enough — no more pinned files")
			break
		}

		for _, u := range oldest {
			if pinnedSize-freed <= targetSize {
				break
			}

			if err := m.ipfs.PinRemove(context.Background(), u.CID); err != nil {
				log.Printf("[pin] failed to unpin %s for eviction: %v", u.CID, err)
				continue
			}
			if err := m.db.MarkUnpinned(u.CID); err != nil {
				log.Printf("[pin] failed to mark %s unpinned: %v", u.CID, err)
				continue
			}

			freed += u.Size
			unpinned++
			log.Printf("[pin] evicted %s (%s)", u.Filename, config.FormatBytes(u.Size))
		}

		pinnedSize, _ = m.db.GetPinnedSize()
	}

	newSize, _ := m.db.GetPinnedSize()
	log.Printf("[pin] eviction done: unpinned %d files, freed %s — now at %s",
		unpinned, config.FormatBytes(freed), config.FormatBytes(newSize))
	return nil
}

func (m *Manager) GetHistory(limit, offset int) ([]store.Upload, error) {
	return m.db.GetUploadHistory(limit, offset)
}

func (m *Manager) GetStats() (count int64, size int64, err error) {
	count, err = m.db.GetPinnedCount()
	if err != nil {
		return
	}
	size, err = m.db.GetPinnedSize()
	return
}
