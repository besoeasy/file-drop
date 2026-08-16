package modules

import (
	"archive/tar"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreArchive(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cid := "bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu"
	has, err := store.HasArchive(cid)
	if err != nil || has {
		t.Fatalf("empty archive should not contain cid: has=%v err=%v", has, err)
	}

	item := ArchiveItem{
		CID:           cid,
		Filename:      "photo.png",
		Mime:          "image/png",
		Size:          42,
		SHA256:        "aa",
		SourceEventID: "evt1",
		Verified:      true,
	}
	if err := store.InsertArchive(item); err != nil {
		t.Fatal(err)
	}
	has, err = store.HasArchive(cid)
	if err != nil || !has {
		t.Fatalf("expected archive hit: has=%v err=%v", has, err)
	}

	got, err := store.GetArchive(cid)
	if err != nil || got == nil {
		t.Fatalf("get: %v %#v", err, got)
	}
	if got.Filename != "photo.png" || !got.Verified || got.Size != 42 {
		t.Fatalf("unexpected item: %+v", got)
	}

	if err := store.SetNostrCursor("npub1abc", 99); err != nil {
		t.Fatal(err)
	}
	ts, err := store.GetNostrCursor("npub1abc")
	if err != nil || ts != 99 {
		t.Fatalf("cursor=%d err=%v", ts, err)
	}
	ts, err = store.GetNostrCursor("missing")
	if err != nil || ts != 0 {
		t.Fatalf("missing cursor=%d err=%v", ts, err)
	}

	n, err := store.IncrArchiveAttempts(cid, "boom")
	if err != nil || n != 1 {
		t.Fatalf("attempts=%d err=%v", n, err)
	}
	n, err = store.IncrArchiveAttempts(cid, "boom2")
	if err != nil || n != 2 {
		t.Fatalf("attempts=%d err=%v", n, err)
	}

	count, size, err := store.GetArchiveStats()
	if err != nil || count != 1 || size != 42 {
		t.Fatalf("stats count=%d size=%d err=%v", count, size, err)
	}

	if err := store.InsertArchive(ArchiveItem{
		CID:          "QmZtmD2qtMeK4B6usxBaTm2WpuFiHg8wf5YDrQj83b27Bm",
		Filename:     "other.png",
		Size:         10,
		SourcePubkey: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertArchiveEvent("evt-a", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d", 1); err != nil {
		t.Fatal(err)
	}
	pc, psz, err := store.GetArchiveStatsForPubkey("3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d")
	if err != nil || pc != 1 || psz != 10 {
		t.Fatalf("pubkey stats count=%d size=%d err=%v", pc, psz, err)
	}
	ev, err := store.CountArchiveEventsForPubkey("3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d")
	if err != nil || ev != 1 {
		t.Fatalf("events=%d err=%v", ev, err)
	}
}

func TestJanitorSkipsArchiveCIDs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	archiveCID := "bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu"
	uploadCID := "QmZtmD2qtMeK4B6usxBaTm2WpuFiHg8wf5YDrQj83b27Bm"
	if err := store.InsertUpload(archiveCID, "kept.png", 10); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertUpload(uploadCID, "evictable.png", 20); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertArchive(ArchiveItem{CID: archiveCID, Filename: "kept.png", Size: 10}); err != nil {
		t.Fatal(err)
	}

	oldest, err := store.GetOldestPinnedFiles(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(oldest) != 1 || oldest[0].CID != uploadCID {
		t.Fatalf("oldest should skip archive cid, got %+v", oldest)
	}

	count, err := store.GetPinnedCount()
	if err != nil {
		t.Fatal(err)
	}
	sz, err := store.GetPinnedSize()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || sz != 20 {
		t.Fatalf("pinned quota should exclude archive overlap count=%d size=%d", count, sz)
	}

	pins, err := store.ListArchivePins()
	if err != nil || len(pins) != 1 || pins[0].CID != archiveCID {
		t.Fatalf("pins=%v err=%v", pins, err)
	}
}

func TestCidAddQuery(t *testing.T) {
	q0 := cidAddQuery("QmZtmD2qtMeK4B6usxBaTm2WpuFiHg8wf5YDrQj83b27Bm", false)
	if !strings.Contains(q0, "cid-version=0") {
		t.Fatalf("v0 query: %s", q0)
	}
	q1 := cidAddQuery("bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu", true)
	if !strings.Contains(q1, "cid-version=1") || !strings.Contains(q1, "wrap-with-directory=true") {
		t.Fatalf("v1 dir query: %s", q1)
	}
}

func TestExtractTarSafe(t *testing.T) {
	cid := "bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu"
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	add := func(name string, body string, dir bool) {
		hdr := &tar.Header{Name: name, Mode: 0o644}
		if dir {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if !dir {
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	add(cid+"/", "", true)
	add(cid+"/hello.txt", "hi", false)
	add("../evil.txt", "nope", false)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	n, err := extractTarSafe(&buf, dest, cid, 1024)
	if err == nil {
		t.Fatal("expected error from path escape in tar")
	}
	_ = n
	_ = os.RemoveAll(dest)
}

func TestArchiveMuxPattern(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("mux pattern panicked: %v", rec)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /archive", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /archive/{cid}", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /archive/{cid}/{path...}", func(http.ResponseWriter, *http.Request) {})
}

func TestMovePath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dest := filepath.Join(dir, "dest")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := movePath(src, dest); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dest)
	if err != nil || string(body) != "hello" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be gone")
	}
}

func TestExtractTarSafeOK(t *testing.T) {
	cid := "bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu"
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: cid + "/hello.txt", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	n, err := extractTarSafe(&buf, dest, cid, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("wrote %d bytes", n)
	}
	body, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hi" {
		t.Fatalf("body = %q", body)
	}
}
