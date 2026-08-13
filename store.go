package main

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
	err := s.db.QueryRow(`SELECT SUM(size) FROM uploads WHERE unpinned = 0`).Scan(&total)
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
	err := s.db.QueryRow(`SELECT COUNT(*) FROM uploads WHERE unpinned = 0`).Scan(&count)
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