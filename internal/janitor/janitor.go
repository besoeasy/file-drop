package janitor

import (
	"context"
	"log"
	"time"

	"github.com/besoeasy/originless/internal/config"
	"github.com/besoeasy/originless/internal/db"
	"github.com/besoeasy/originless/internal/ipfs"
)

type Manager struct {
	store     *db.Store
	ipfs      *ipfs.Client
	limit     int64
	threshold float64
	expiry    time.Duration
}

func New(store *db.Store, ipfsClient *ipfs.Client, limit int64) *Manager {
	return &Manager{
		store:     store,
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
		log.Printf("[janitor] failed to pin %s: %v", cid, err)
		return err
	}

	if err := m.store.InsertUpload(cid, filename, size); err != nil {
		log.Printf("[janitor] db insert failed for %s: %v — rolling back pin", cid, err)
		if rbErr := m.ipfs.PinRemove(context.Background(), cid); rbErr != nil {
			log.Printf("[janitor] rollback unpin failed for %s: %v", cid, rbErr)
		}
		return err
	}

	pinnedSize, _ := m.store.GetPinnedSize()
	log.Printf("[janitor] pinned %s (%s) — total pinned: %s",
		cid, config.FormatBytes(size), config.FormatBytes(pinnedSize))

	if float64(pinnedSize) > float64(m.limit)*m.threshold {
		log.Printf("[janitor] storage exceeds %d%% threshold, evicting oldest", config.PinThreshold)
		if err := m.EvictOldest(); err != nil {
			log.Printf("[janitor] eviction failed: %v", err)
		}
	}

	return nil
}

func (m *Manager) Reconcile() error {
	log.Printf("[janitor] starting startup reconciliation...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ipfsPins, err := m.ipfs.PinList(ctx)
	if err != nil {
		log.Printf("[janitor] failed to list IPFS pins: %v (skipping reconciliation)", err)
		return err
	}

	dbCIDs, err := m.store.GetAllTrackedCIDs()
	if err != nil {
		log.Printf("[janitor] failed to get tracked CIDs: %v", err)
		return err
	}

	var orphaned int
	for cid := range ipfsPins {
		if _, tracked := dbCIDs[cid]; !tracked {
			size, err := m.ipfs.ObjectStat(ctx, cid)
			if err != nil {
				log.Printf("[janitor] failed to stat orphan %s: %v (using size=0)", cid, err)
				size = 0
			}
			if err := m.store.InsertOrphaned(cid, size); err != nil {
				log.Printf("[janitor] failed to import orphan %s: %v", cid, err)
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
		if err := m.store.MarkMissingAsUnpinned(missing); err != nil {
			log.Printf("[janitor] failed to mark missing: %v", err)
		}
	}

	pinnedCount, _ := m.store.GetPinnedCount()
	pinnedSize, _ := m.store.GetPinnedSize()
	log.Printf("[janitor] reconciliation done: %d orphaned imported, %d missing marked — %d pinned (%s)",
		orphaned, len(missing), pinnedCount, config.FormatBytes(pinnedSize))

	return nil
}

func (m *Manager) Run(ctx context.Context, interval time.Duration) {
	log.Printf("[janitor] started (interval: %s, expiry: %s, threshold: %d%%)",
		interval, m.expiry, config.PinThreshold)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[janitor] stopped")
			return
		case <-ticker.C:
			if err := m.UnpinExpired(); err != nil {
				log.Printf("[janitor] expired unpin cycle error: %v", err)
			}
			if err := m.CheckThreshold(); err != nil {
				log.Printf("[janitor] threshold check error: %v", err)
			}
		}
	}
}

func (m *Manager) UnpinExpired() error {
	expired, err := m.store.GetExpiredUnpins(m.expiry)
	if err != nil {
		return err
	}

	if len(expired) == 0 {
		return nil
	}

	log.Printf("[janitor] unpinning %d expired files", len(expired))

	var unpinned int
	for _, u := range expired {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := m.ipfs.PinRemove(ctx, u.CID); err != nil {
			cancel()
			log.Printf("[janitor] failed to unpin %s: %v", u.CID, err)
			continue
		}
		cancel()
		if err := m.store.MarkUnpinned(u.CID); err != nil {
			log.Printf("[janitor] failed to mark unpinned %s: %v", u.CID, err)
			continue
		}
		unpinned++
	}

	pinnedSize, _ := m.store.GetPinnedSize()
	log.Printf("[janitor] unpinned %d expired files — pinned: %s", unpinned, config.FormatBytes(pinnedSize))
	return nil
}

func (m *Manager) CheckThreshold() error {
	pinnedSize, err := m.store.GetPinnedSize()
	if err != nil {
		return err
	}

	if float64(pinnedSize) <= float64(m.limit)*m.threshold {
		return nil
	}

	log.Printf("[janitor] pinned size %s exceeds %d%% of %s, evicting",
		config.FormatBytes(pinnedSize), config.PinThreshold, config.FormatBytes(m.limit))

	return m.EvictOldest()
}

func (m *Manager) EvictOldest() error {
	pinnedSize, err := m.store.GetPinnedSize()
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
		oldest, err := m.store.GetOldestPinnedFiles(50)
		if err != nil {
			return err
		}
		if len(oldest) == 0 {
			log.Printf("[janitor] warning: cannot evict enough — no more pinned files")
			break
		}

		for _, u := range oldest {
			if pinnedSize-freed <= targetSize {
				break
			}

			if err := m.ipfs.PinRemove(context.Background(), u.CID); err != nil {
				log.Printf("[janitor] failed to unpin %s for eviction: %v", u.CID, err)
				continue
			}
			if err := m.store.MarkUnpinned(u.CID); err != nil {
				log.Printf("[janitor] failed to mark %s unpinned: %v", u.CID, err)
				continue
			}

			freed += u.Size
			unpinned++
			log.Printf("[janitor] evicted %s (%s)", u.Filename, config.FormatBytes(u.Size))
		}

		pinnedSize, _ = m.store.GetPinnedSize()
	}

	newSize, _ := m.store.GetPinnedSize()
	log.Printf("[janitor] eviction done: unpinned %d files, freed %s — now at %s",
		unpinned, config.FormatBytes(freed), config.FormatBytes(newSize))
	return nil
}

func (m *Manager) GetHistory(limit, offset int) ([]db.Upload, error) {
	return m.store.GetUploadHistory(limit, offset)
}

func (m *Manager) GetStats() (count int64, size int64, err error) {
	count, err = m.store.GetPinnedCount()
	if err != nil {
		return
	}
	size, err = m.store.GetPinnedSize()
	return
}
