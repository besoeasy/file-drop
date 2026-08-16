package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const maxArchiveAttempts = 5

type Archiver struct {
	store     *Store
	ipfs      *Client
	dir       string
	http      *http.Client
	running   atomic.Bool
	saved     atomic.Int64
	savedSize atomic.Int64
	errors    atomic.Int64
}

func NewArchiver(store *Store, ipfsClient *Client, dir string) *Archiver {
	return &Archiver{
		store: store,
		ipfs:  ipfsClient,
		dir:   dir,
		http:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (a *Archiver) Run(ctx context.Context, interval time.Duration) {
	if len(NostrNpubs) == 0 {
		log.Printf("[archive] no NOSTR_NPUBS configured, archiver idle")
		return
	}
	log.Printf("[archive] started dir=%s interval=%s npubs=%d", a.dir, interval, len(NostrNpubs))
	a.Cycle(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[archive] stopped")
			return
		case <-ticker.C:
			a.Cycle(ctx)
		}
	}
}

func (a *Archiver) Reconcile() {
	entries, err := os.ReadDir(a.dir)
	if err != nil {
		log.Printf("[archive] reconcile skipped: %v", err)
		return
	}
	known, err := a.store.ListArchiveCIDs()
	if err != nil {
		log.Printf("[archive] reconcile list failed: %v", err)
		return
	}
	var imported int
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !ValidCID(name) {
			continue
		}
		if known[name] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		if e.IsDir() {
			size, _ = dirSize(filepath.Join(a.dir, name))
		}
		item := ArchiveItem{
			CID:      name,
			Filename: name,
			Size:     size,
			IsDir:    e.IsDir(),
			Verified: true,
		}
		if err := a.store.InsertArchive(item); err != nil {
			log.Printf("[archive] reconcile insert %s: %v", name, err)
			continue
		}
		imported++
	}
	count, total, _ := a.store.GetArchiveStats()
	log.Printf("[archive] reconcile done imported=%d items=%d size=%s", imported, count, FormatBytes(total))
}

func (a *Archiver) Cycle(ctx context.Context) {
	if len(NostrNpubs) == 0 {
		return
	}
	if !a.running.CompareAndSwap(false, true) {
		log.Printf("[archive] cycle already running, skip")
		return
	}
	defer a.running.Store(false)

	log.Printf("[archive] scanning %d npubs", len(NostrNpubs))

	for _, npub := range NostrNpubs {
		if ctx.Err() != nil {
			return
		}
		if err := a.scanNpub(ctx, npub); err != nil {
			log.Printf("[archive] npub %s: %v", npub, err)
			a.errors.Add(1)
		}
	}

	count, total, _ := a.store.GetArchiveStats()
	log.Printf("[archive] cycle done items=%d size=%s", count, FormatBytes(total))
}

func (a *Archiver) scanNpub(ctx context.Context, npub string) error {
	cursor, err := a.store.GetNostrCursor(npub)
	if err != nil {
		return err
	}

	posts, err := FetchAllUserPosts(ctx, npub, FetchOptions{
		Kinds:      ArchiveFetchKinds,
		Since:      cursor,
		PageSize:   250,
		MaxPages:   50,
		ReqTimeout: 8 * time.Second,
	})
	if err != nil {
		return err
	}

	sort.Slice(posts, func(i, j int) bool {
		if posts[i].CreatedAt == posts[j].CreatedAt {
			return posts[i].ID < posts[j].ID
		}
		return posts[i].CreatedAt < posts[j].CreatedAt
	})

	log.Printf("[archive] npub=%s events=%d since=%d", npub, len(posts), cursor)

	for _, evt := range posts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		done, err := a.store.HasArchiveEvent(evt.ID)
		if err != nil {
			return err
		}
		if done {
			if evt.CreatedAt > cursor {
				cursor = evt.CreatedAt
				_ = a.store.SetNostrCursor(npub, cursor)
			}
			continue
		}

		refs := ExtractIPFSRefs(evt)
		blocked := false
		for _, ref := range refs {
			if err := a.archiveRef(ctx, npub, evt, ref); err != nil {
				log.Printf("[archive] cid=%s event=%s: %v", ref.CID, evt.ID, err)
				a.errors.Add(1)
				blocked = true
				break
			}
		}
		if blocked {
			return fmt.Errorf("download failed for event %s, will retry", evt.ID)
		}
		if err := a.store.InsertArchiveEvent(evt.ID, evt.PubKey, evt.CreatedAt); err != nil {
			return err
		}
		if evt.CreatedAt > cursor {
			cursor = evt.CreatedAt
			if err := a.store.SetNostrCursor(npub, cursor); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Archiver) archiveRef(ctx context.Context, npub string, evt NostrEvent, ref IPFSRef) error {
	exists, err := a.store.HasArchive(ref.CID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	attempts, err := a.store.GetArchiveAttempts(ref.CID)
	if err != nil {
		return err
	}
	if attempts >= maxArchiveAttempts {
		log.Printf("[archive] giving up cid=%s after %d attempts", ref.CID, attempts)
		return nil
	}

	dest := filepath.Join(a.dir, ref.CID)
	tmp := filepath.Join(UploadTempDir, "archive-"+ref.CID)

	_ = os.RemoveAll(tmp)
	_ = os.RemoveAll(tmp + ".dir")

	size, isDir, sha, verified, err := a.download(ctx, ref, tmp)
	if err != nil {
		_, _ = a.store.IncrArchiveAttempts(ref.CID, err.Error())
		_ = os.RemoveAll(tmp)
		_ = os.RemoveAll(tmp + ".dir")
		return err
	}

	if err := movePath(tmp, dest); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("move into archive: %w", err)
	}

	item := ArchiveItem{
		CID:           ref.CID,
		Filename:      firstNonEmpty(ref.Filename, ref.CID),
		Mime:          ref.Mime,
		Size:          size,
		SHA256:        sha,
		SourceEventID: evt.ID,
		SourcePubkey:  evt.PubKey,
		SourceURL:     ref.URL,
		Verified:      verified,
		IsDir:         isDir,
	}
	if err := a.store.InsertArchive(item); err != nil {
		return err
	}

	a.saved.Add(1)
	a.savedSize.Add(size)
	log.Printf("[archive] saved cid=%s size=%s verified=%v dir=%v npub=%s event=%s",
		ref.CID, FormatBytes(size), verified, isDir, npub, evt.ID)
	return nil
}

func (a *Archiver) download(ctx context.Context, ref IPFSRef, dest string) (size int64, isDir bool, sha string, verified bool, err error) {
	size, isDir, sha, err = a.downloadKubo(ctx, ref.CID, dest)
	if err == nil {
		if mismatch := shaMismatch(ref.SHA256, sha, isDir); mismatch != "" {
			_ = os.RemoveAll(dest)
			return 0, false, "", false, fmt.Errorf("%s", mismatch)
		}
		return size, isDir, sha, true, nil
	}
	kuboErr := err

	urls := gatewayURLs(ref)
	if len(urls) == 0 {
		return 0, false, "", false, fmt.Errorf("kubo fetch failed: %w", kuboErr)
	}

	var last error
	for _, u := range urls {
		size, sha, err = a.downloadHTTP(ctx, u, dest, FileLimit)
		if err != nil {
			last = err
			_ = os.RemoveAll(dest)
			continue
		}
		if mismatch := shaMismatch(ref.SHA256, sha, false); mismatch != "" {
			_ = os.RemoveAll(dest)
			last = fmt.Errorf("%s", mismatch)
			continue
		}
		verified = ref.SHA256 != "" && strings.EqualFold(ref.SHA256, sha)
		if !verified {
			log.Printf("[archive] gateway fetch cid=%s unverified (no sha256 tag, kubo: %v)", ref.CID, kuboErr)
		}
		return size, false, sha, verified, nil
	}
	if last == nil {
		last = kuboErr
	}
	return 0, false, "", false, fmt.Errorf("kubo: %v; gateway: %w", kuboErr, last)
}

func (a *Archiver) downloadKubo(ctx context.Context, cid, dest string) (size int64, isDir bool, sha string, err error) {
	size, err = a.ipfs.CatToFile(ctx, cid, dest, FileLimit)
	if err == nil {
		sha, err = fileSHA256(dest)
		return size, false, sha, err
	}
	_ = os.Remove(dest)
	if !isDirectoryError(err) {
		return 0, false, "", err
	}
	dirDest := dest + ".dir"
	_ = os.RemoveAll(dirDest)
	size, err = a.ipfs.GetDirToPath(ctx, cid, dirDest, FileLimit)
	if err != nil {
		_ = os.RemoveAll(dirDest)
		return 0, false, "", err
	}
	if err := movePath(dirDest, dest); err != nil {
		return 0, false, "", err
	}
	return size, true, "", nil
}

func (a *Archiver) downloadHTTP(ctx context.Context, rawURL, dest string, maxBytes int64) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}

	f, err := os.Create(dest)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return written, "", err
	}
	if written > maxBytes {
		return written, "", fmt.Errorf("content exceeds max size %d", maxBytes)
	}
	return written, hex.EncodeToString(h.Sum(nil)), nil
}

func (a *Archiver) GetStats() (count, size int64, err error) {
	return a.store.GetArchiveStats()
}

func (a *Archiver) List(limit, offset int) ([]ArchiveItem, error) {
	return a.store.ListArchive(limit, offset)
}

func (a *Archiver) Get(cid string) (*ArchiveItem, error) {
	return a.store.GetArchive(cid)
}

func (a *Archiver) Path(cid string) string {
	return filepath.Join(a.dir, cid)
}

func shaMismatch(expected, actual string, isDir bool) string {
	if expected == "" || isDir {
		return ""
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Sprintf("sha256 mismatch: expected %s got %s", expected, actual)
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func gatewayURLs(ref IPFSRef) []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(u string) {
		if u == "" || seen[u] {
			return
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return
		}
		seen[u] = true
		urls = append(urls, u)
	}
	add(ref.URL)
	add("https://ipfs.io/ipfs/" + ref.CID)
	add("https://dweb.link/ipfs/" + ref.CID)
	return urls
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func movePath(src, dest string) error {
	_ = os.RemoveAll(dest)
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := copyDir(src, dest); err != nil {
			_ = os.RemoveAll(dest)
			return err
		}
		return os.RemoveAll(src)
	}
	if err := copyFile(src, dest); err != nil {
		_ = os.Remove(dest)
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
