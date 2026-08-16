package modules

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type Upload struct {
	ID         int64      `json:"id"`
	CID        string     `json:"cid"`
	Filename   string     `json:"filename"`
	Size       int64      `json:"size"`
	CreatedAt  time.Time  `json:"created_at"`
	Unpinned   bool       `json:"unpinned"`
	UnpinnedAt *time.Time `json:"unpinned_at,omitempty"`
}

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	log.Printf("[db] database ready at %s", dbPath)
	return &Store{db: sqlDB}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS uploads (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			cid         TEXT NOT NULL UNIQUE,
			filename    TEXT NOT NULL,
			size        INTEGER NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			unpinned    BOOLEAN DEFAULT 0,
			unpinned_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_uploads_created ON uploads(created_at);
		CREATE INDEX IF NOT EXISTS idx_uploads_unpinned ON uploads(unpinned);

		CREATE TABLE IF NOT EXISTS archive (
			cid             TEXT PRIMARY KEY,
			filename        TEXT NOT NULL DEFAULT '',
			mime            TEXT NOT NULL DEFAULT '',
			size            INTEGER NOT NULL,
			sha256          TEXT NOT NULL DEFAULT '',
			source_event_id TEXT NOT NULL DEFAULT '',
			source_pubkey   TEXT NOT NULL DEFAULT '',
			source_url      TEXT NOT NULL DEFAULT '',
			verified        INTEGER NOT NULL DEFAULT 0,
			is_dir          INTEGER NOT NULL DEFAULT 0,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_archive_created ON archive(created_at);

		CREATE TABLE IF NOT EXISTS archive_events (
			event_id   TEXT PRIMARY KEY,
			pubkey     TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS nostr_cursors (
			npub             TEXT PRIMARY KEY,
			last_created_at  INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS archive_attempts (
			cid        TEXT PRIMARY KEY,
			attempts   INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (s *Store) InsertUpload(cid, filename string, size int64) error {
	_, err := s.db.Exec(
		`INSERT INTO uploads (cid, filename, size) VALUES (?, ?, ?)`,
		cid, filename, size,
	)
	if err != nil {
		return fmt.Errorf("insert upload %s: %w", cid, err)
	}
	return nil
}

func (s *Store) MarkUnpinned(cid string) error {
	_, err := s.db.Exec(
		`UPDATE uploads SET unpinned = 1, unpinned_at = CURRENT_TIMESTAMP WHERE cid = ? AND unpinned = 0`,
		cid,
	)
	if err != nil {
		return fmt.Errorf("mark unpinned %s: %w", cid, err)
	}
	return nil
}

func (s *Store) GetExpiredUnpins(expiry time.Duration) ([]Upload, error) {
	cutoff := time.Now().Add(-expiry)
	rows, err := s.db.Query(
		`SELECT id, cid, filename, size, created_at, unpinned, unpinned_at
		 FROM uploads
		 WHERE unpinned = 0 AND created_at < ?
		 AND cid NOT IN (SELECT cid FROM archive)
		 ORDER BY created_at ASC`,
		cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUploads(rows)
}

func (s *Store) GetOldestPinnedFiles(limit int) ([]Upload, error) {
	rows, err := s.db.Query(
		`SELECT id, cid, filename, size, created_at, unpinned, unpinned_at
		 FROM uploads
		 WHERE unpinned = 0
		 AND cid NOT IN (SELECT cid FROM archive)
		 ORDER BY created_at ASC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUploads(rows)
}

func (s *Store) GetPinnedSize() (int64, error) {
	var total sql.NullInt64
	err := s.db.QueryRow(`SELECT SUM(size) FROM uploads WHERE unpinned = 0 AND cid NOT IN (SELECT cid FROM archive)`).Scan(&total)
	if err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

func (s *Store) GetPinnedCount() (int64, error) {
	var count sql.NullInt64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM uploads WHERE unpinned = 0 AND cid NOT IN (SELECT cid FROM archive)`).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count.Valid {
		return count.Int64, nil
	}
	return 0, nil
}

func (s *Store) GetUploadHistory(limit, offset int) ([]Upload, error) {
	rows, err := s.db.Query(
		`SELECT id, cid, filename, size, created_at, unpinned, unpinned_at
		 FROM uploads
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUploads(rows)
}

func (s *Store) GetUploadByCID(cid string) (*Upload, error) {
	var u Upload
	err := s.db.QueryRow(
		`SELECT id, cid, filename, size, created_at, unpinned, unpinned_at
		 FROM uploads WHERE cid = ?`, cid,
	).Scan(&u.ID, &u.CID, &u.Filename, &u.Size, &u.CreatedAt, &u.Unpinned, &u.UnpinnedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetAllTrackedCIDs() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT cid, unpinned FROM uploads`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var cid string
		var unpinned bool
		if err := rows.Scan(&cid, &unpinned); err != nil {
			return nil, err
		}
		result[cid] = unpinned
	}
	return result, nil
}

func (s *Store) InsertOrphaned(cid string, size int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO uploads (cid, filename, size) VALUES (?, ?, ?)`,
		cid, cid, size,
	)
	return err
}

func (s *Store) MarkMissingAsUnpinned(cids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE uploads SET unpinned = 1, unpinned_at = CURRENT_TIMESTAMP WHERE cid = ? AND unpinned = 0`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cid := range cids {
		if _, err := stmt.Exec(cid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanUploads(rows *sql.Rows) ([]Upload, error) {
	var uploads []Upload
	for rows.Next() {
		var u Upload
		if err := rows.Scan(&u.ID, &u.CID, &u.Filename, &u.Size, &u.CreatedAt, &u.Unpinned, &u.UnpinnedAt); err != nil {
			return nil, err
		}
		uploads = append(uploads, u)
	}
	return uploads, rows.Err()
}

type ArchiveItem struct {
	CID           string    `json:"cid"`
	Filename      string    `json:"filename"`
	Mime          string    `json:"mime"`
	Size          int64     `json:"size"`
	SHA256        string    `json:"sha256,omitempty"`
	SourceEventID string    `json:"source_event_id,omitempty"`
	SourcePubkey  string    `json:"source_pubkey,omitempty"`
	SourceURL     string    `json:"source_url,omitempty"`
	Verified      bool      `json:"verified"`
	IsDir         bool      `json:"is_dir"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) HasArchive(cid string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM archive WHERE cid = ?`, cid).Scan(&n)
	return n > 0, err
}

func (s *Store) InsertArchive(item ArchiveItem) error {
	verified := 0
	if item.Verified {
		verified = 1
	}
	isDir := 0
	if item.IsDir {
		isDir = 1
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO archive
		 (cid, filename, mime, size, sha256, source_event_id, source_pubkey, source_url, verified, is_dir)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.CID, item.Filename, item.Mime, item.Size, item.SHA256,
		item.SourceEventID, item.SourcePubkey, item.SourceURL, verified, isDir,
	)
	return err
}

func (s *Store) GetArchive(cid string) (*ArchiveItem, error) {
	var item ArchiveItem
	var verified, isDir int
	err := s.db.QueryRow(
		`SELECT cid, filename, mime, size, sha256, source_event_id, source_pubkey, source_url, verified, is_dir, created_at
		 FROM archive WHERE cid = ?`, cid,
	).Scan(&item.CID, &item.Filename, &item.Mime, &item.Size, &item.SHA256,
		&item.SourceEventID, &item.SourcePubkey, &item.SourceURL, &verified, &isDir, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.Verified = verified != 0
	item.IsDir = isDir != 0
	return &item, nil
}

func (s *Store) ListArchive(limit, offset int) ([]ArchiveItem, error) {
	rows, err := s.db.Query(
		`SELECT cid, filename, mime, size, sha256, source_event_id, source_pubkey, source_url, verified, is_dir, created_at
		 FROM archive ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ArchiveItem
	for rows.Next() {
		var item ArchiveItem
		var verified, isDir int
		if err := rows.Scan(&item.CID, &item.Filename, &item.Mime, &item.Size, &item.SHA256,
			&item.SourceEventID, &item.SourcePubkey, &item.SourceURL, &verified, &isDir, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Verified = verified != 0
		item.IsDir = isDir != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetArchiveStats() (count, size int64, err error) {
	var c, ssum sql.NullInt64
	err = s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size), 0) FROM archive`).Scan(&c, &ssum)
	if err != nil {
		return 0, 0, err
	}
	return c.Int64, ssum.Int64, nil
}

func (s *Store) GetArchiveStatsForPubkey(pubkey string) (count, size int64, err error) {
	var c, ssum sql.NullInt64
	err = s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(size), 0) FROM archive WHERE source_pubkey = ?`,
		pubkey,
	).Scan(&c, &ssum)
	if err != nil {
		return 0, 0, err
	}
	return c.Int64, ssum.Int64, nil
}

func (s *Store) CountArchiveEventsForPubkey(pubkey string) (int64, error) {
	var n sql.NullInt64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM archive_events WHERE pubkey = ?`, pubkey).Scan(&n)
	return n.Int64, err
}

func (s *Store) HasArchiveEvent(eventID string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM archive_events WHERE event_id = ?`, eventID).Scan(&n)
	return n > 0, err
}

func (s *Store) InsertArchiveEvent(eventID, pubkey string, createdAt int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO archive_events (event_id, pubkey, created_at) VALUES (?, ?, ?)`,
		eventID, pubkey, createdAt,
	)
	return err
}

func (s *Store) GetNostrCursor(npub string) (int64, error) {
	var ts int64
	err := s.db.QueryRow(`SELECT last_created_at FROM nostr_cursors WHERE npub = ?`, npub).Scan(&ts)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return ts, err
}

func (s *Store) SetNostrCursor(npub string, ts int64) error {
	_, err := s.db.Exec(
		`INSERT INTO nostr_cursors (npub, last_created_at) VALUES (?, ?)
		 ON CONFLICT(npub) DO UPDATE SET last_created_at = excluded.last_created_at`,
		npub, ts,
	)
	return err
}

func (s *Store) GetArchiveAttempts(cid string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT attempts FROM archive_attempts WHERE cid = ?`, cid).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (s *Store) IncrArchiveAttempts(cid, lastError string) (int, error) {
	_, err := s.db.Exec(
		`INSERT INTO archive_attempts (cid, attempts, last_error, updated_at) VALUES (?, 1, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(cid) DO UPDATE SET attempts = attempts + 1, last_error = excluded.last_error, updated_at = CURRENT_TIMESTAMP`,
		cid, lastError,
	)
	if err != nil {
		return 0, err
	}
	return s.GetArchiveAttempts(cid)
}

func (s *Store) ListArchiveCIDs() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT cid FROM archive`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return nil, err
		}
		result[cid] = true
	}
	return result, rows.Err()
}

func (s *Store) ListArchivePins() ([]ArchiveItem, error) {
	rows, err := s.db.Query(`SELECT cid, is_dir FROM archive ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ArchiveItem
	for rows.Next() {
		var item ArchiveItem
		var isDir int
		if err := rows.Scan(&item.CID, &isDir); err != nil {
			return nil, err
		}
		item.IsDir = isDir != 0
		items = append(items, item)
	}
	return items, rows.Err()
}
