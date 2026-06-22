package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go, cgo-free driver registered as "sqlite"
)

// creatingMaxAge is how long an unfinished ("creating") send may linger before GC
// reaps it. Kept short to bound the disk a stalled/abusive uploader can hold: a
// whole send is <= 25 MiB and uploads in seconds, so 15m is ample for legit use.
const creatingMaxAge = 15 * time.Minute

// SQLiteState is the StateStore backed by a single cgo-free SQLite database. It
// serializes writes via SetMaxOpenConns(1), which removes SQLITE_BUSY at the low
// QPS this server targets while keeping the atomic one-time redeem trivially correct.
type SQLiteState struct {
	db       *sql.DB
	now      func() time.Time
	grantTTL time.Duration
}

// OpenSQLiteState opens (creating if needed) the database at path and migrates the
// schema. now may be nil (defaults to time.Now); tests inject it to drive expiry.
func OpenSQLiteState(path string, grantTTL time.Duration, now func() time.Time) (*SQLiteState, error) {
	if now == nil {
		now = time.Now
	}
	// SQLite will not create missing parent directories, so the default
	// SEND_DB_PATH=data/sends.db must work on a fresh run (mirrors fsblob).
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}
	s := &SQLiteState{db: db, now: now, grantTTL: grantTTL}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteState) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sends (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL CHECK (status IN ('creating','finalized','expired','deleted')),
			one_time INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			finalized_at INTEGER,
			expires_at INTEGER NOT NULL,
			consumed_at INTEGER,
			total_encrypted_size INTEGER NOT NULL DEFAULT 0,
			part_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS send_parts (
			send_id TEXT NOT NULL REFERENCES sends(id) ON DELETE CASCADE,
			part_id TEXT NOT NULL,
			storage_key TEXT NOT NULL,
			encrypted_size INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			uploaded_at INTEGER,
			PRIMARY KEY (send_id, part_id)
		)`,
		`CREATE TABLE IF NOT EXISTS redeem_grants (
			token_hash TEXT PRIMARY KEY,
			send_id TEXT NOT NULL REFERENCES sends(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			client_ip_hash TEXT,
			user_agent_hash TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_grants_send ON redeem_grants(send_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sends_expires ON sends(expires_at)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Close checkpoints the WAL (so a backup of the .db file is complete) and closes.
func (s *SQLiteState) Close() error {
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return s.db.Close()
}

func (s *SQLiteState) CreateSend(ctx context.Context, in CreateSendInput) (SendRecord, error) {
	id := NewSendID()
	now := s.now()
	expiresAt := now.Add(in.TTL)
	var total int64
	for _, p := range in.Parts {
		total += p.EncryptedSize
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SendRecord{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful Commit

	oneTime := boolToInt(in.OneTime)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sends (id, status, one_time, created_at, expires_at, total_encrypted_size, part_count)
		 VALUES (?, 'creating', ?, ?, ?, ?, ?)`,
		id, oneTime, now.Unix(), expiresAt.Unix(), total, len(in.Parts),
	); err != nil {
		return SendRecord{}, fmt.Errorf("insert send: %w", err)
	}
	for _, p := range in.Parts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO send_parts (send_id, part_id, storage_key, encrypted_size, sha256)
			 VALUES (?, ?, ?, ?, ?)`,
			id, p.PartID, StorageKey(id, p.PartID), p.EncryptedSize, p.SHA256,
		); err != nil {
			return SendRecord{}, fmt.Errorf("insert part: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return SendRecord{}, fmt.Errorf("commit: %w", err)
	}
	return SendRecord{ID: id, OneTime: in.OneTime, ExpiresAt: expiresAt, Parts: in.Parts}, nil
}

func (s *SQLiteState) SendStatusOf(ctx context.Context, sendID string) (SendStatus, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM sends WHERE id = ?`, sendID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("status: %w", err)
	}
	return SendStatus(status), nil
}

func (s *SQLiteState) DeclaredPart(ctx context.Context, sendID, partID string) (PartMeta, bool, error) {
	var (
		size     int64
		sha      string
		uploaded sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT encrypted_size, sha256, uploaded_at FROM send_parts WHERE send_id = ? AND part_id = ?`,
		sendID, partID,
	).Scan(&size, &sha, &uploaded)
	if errors.Is(err, sql.ErrNoRows) {
		return PartMeta{}, false, ErrUnknownPart
	}
	if err != nil {
		return PartMeta{}, false, fmt.Errorf("declared part: %w", err)
	}
	return PartMeta{PartID: partID, EncryptedSize: size, SHA256: sha}, uploaded.Valid, nil
}

func (s *SQLiteState) MarkUploaded(ctx context.Context, sendID, partID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE send_parts SET uploaded_at = ? WHERE send_id = ? AND part_id = ?`,
		s.now().Unix(), sendID, partID,
	)
	if err != nil {
		return fmt.Errorf("mark uploaded: %w", err)
	}
	return nil
}

func (s *SQLiteState) FinalizeSend(ctx context.Context, sendID string) error {
	status, err := s.SendStatusOf(ctx, sendID)
	if err != nil {
		return err
	}
	switch status {
	case SendStatusFinalized:
		return nil // idempotent: re-finalizing a complete send is a no-op
	case SendStatusCreating:
		// proceed
	default:
		return ErrConflict
	}

	var missing int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM send_parts WHERE send_id = ? AND uploaded_at IS NULL`, sendID,
	).Scan(&missing); err != nil {
		return fmt.Errorf("count missing: %w", err)
	}
	if missing > 0 {
		return ErrIncomplete
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE sends SET status = 'finalized', finalized_at = ? WHERE id = ? AND status = 'creating'`,
		s.now().Unix(), sendID,
	)
	if err != nil {
		return fmt.Errorf("finalize: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

// RedeemSend opens a download grant. For one-time sends the public link is consumed
// by a single atomic conditional UPDATE — the only consume path (never read-then-write).
func (s *SQLiteState) RedeemSend(ctx context.Context, sendID string) (RedeemGrant, error) {
	now := s.now()

	var (
		oneTime   int
		status    string
		expiresAt int64
		partCount int
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT one_time, status, expires_at, part_count FROM sends WHERE id = ?`, sendID,
	).Scan(&oneTime, &status, &expiresAt, &partCount)
	if errors.Is(err, sql.ErrNoRows) {
		return RedeemGrant{}, ErrNotFound
	}
	if err != nil {
		return RedeemGrant{}, fmt.Errorf("load send: %w", err)
	}
	if SendStatus(status) != SendStatusFinalized {
		return RedeemGrant{}, ErrNotFinalized
	}
	if expiresAt <= now.Unix() {
		return RedeemGrant{}, ErrExpired
	}

	if oneTime == 1 {
		res, err := s.db.ExecContext(ctx,
			`UPDATE sends SET consumed_at = ?
			 WHERE id = ? AND one_time = 1 AND consumed_at IS NULL
			   AND status = 'finalized' AND expires_at > ?`,
			now.Unix(), sendID, now.Unix(),
		)
		if err != nil {
			return RedeemGrant{}, fmt.Errorf("consume: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// Lost the race or already consumed (expiry was ruled out above).
			return RedeemGrant{}, ErrAlreadyRedeemed
		}
	}

	token := NewRedeemToken()
	grantExp := now.Add(s.grantTTL)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO redeem_grants (token_hash, send_id, created_at, expires_at)
		 VALUES (?, ?, ?, ?)`,
		HashToken(token), sendID, now.Unix(), grantExp.Unix(),
	); err != nil {
		return RedeemGrant{}, fmt.Errorf("insert grant: %w", err)
	}

	parts, err := s.partsBrief(ctx, sendID, partCount)
	if err != nil {
		return RedeemGrant{}, err
	}
	return RedeemGrant{Token: token, ExpiresAt: grantExp, Parts: parts}, nil
}

func (s *SQLiteState) ValidateGrant(ctx context.Context, sendID, tokenHash string) error {
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM redeem_grants WHERE token_hash = ? AND send_id = ?`,
		tokenHash, sendID,
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidGrant
	}
	if err != nil {
		return fmt.Errorf("validate grant: %w", err)
	}
	if expiresAt <= s.now().Unix() {
		return ErrInvalidGrant
	}
	return nil
}

func (s *SQLiteState) GetSendMeta(ctx context.Context, sendID string) (SendMeta, error) {
	var (
		status    string
		oneTime   int
		expiresAt int64
		partCount int
		total     int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT status, one_time, expires_at, part_count, total_encrypted_size FROM sends WHERE id = ?`,
		sendID,
	).Scan(&status, &oneTime, &expiresAt, &partCount, &total)
	if errors.Is(err, sql.ErrNoRows) {
		return SendMeta{}, ErrNotFound
	}
	if err != nil {
		return SendMeta{}, fmt.Errorf("meta: %w", err)
	}
	parts, err := s.partsFull(ctx, sendID, partCount)
	if err != nil {
		return SendMeta{}, err
	}
	return SendMeta{
		ID:                 sendID,
		Status:             SendStatus(status),
		OneTime:            oneTime == 1,
		ExpiresAt:          time.Unix(expiresAt, 0).UTC(),
		PartCount:          partCount,
		TotalEncryptedSize: total,
		Parts:              parts,
	}, nil
}

// DeleteExpired removes expired sends, consumed one-time sends past their grant
// window, and unfinished sends older than creatingMaxAge, returning the count of
// deleted sends and the storage keys to purge from the blob store.
func (s *SQLiteState) DeleteExpired(ctx context.Context, now time.Time) (int, []string, error) {
	consumedThresh := now.Add(-s.grantTTL).Unix()
	creatingThresh := now.Add(-creatingMaxAge).Unix()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM sends
		   WHERE expires_at <= ?
		      OR (one_time = 1 AND consumed_at IS NOT NULL AND consumed_at <= ?)
		      OR (status = 'creating' AND created_at <= ?)`,
		now.Unix(), consumedThresh, creatingThresh,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("select expired: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("scan id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("rows: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		return 0, nil, nil
	}

	keys, err := s.storageKeysFor(ctx, ids)
	if err != nil {
		return 0, nil, err
	}
	placeholders, args := inClause(ids)
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM sends WHERE id IN (`+placeholders+`)`, args...,
	); err != nil {
		return 0, nil, fmt.Errorf("delete sends: %w", err)
	}
	return len(ids), keys, nil
}

func (s *SQLiteState) LiveStorageKeys(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT storage_key FROM send_parts`)
	if err != nil {
		return nil, fmt.Errorf("live keys: %w", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *SQLiteState) partsBrief(ctx context.Context, sendID string, hint int) ([]PartMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT part_id, encrypted_size FROM send_parts WHERE send_id = ? ORDER BY part_id`, sendID)
	if err != nil {
		return nil, fmt.Errorf("parts brief: %w", err)
	}
	defer rows.Close()
	parts := make([]PartMeta, 0, hint)
	for rows.Next() {
		var p PartMeta
		if err := rows.Scan(&p.PartID, &p.EncryptedSize); err != nil {
			return nil, fmt.Errorf("scan part: %w", err)
		}
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

func (s *SQLiteState) partsFull(ctx context.Context, sendID string, hint int) ([]PartMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT part_id, encrypted_size, sha256 FROM send_parts WHERE send_id = ? ORDER BY part_id`, sendID)
	if err != nil {
		return nil, fmt.Errorf("parts full: %w", err)
	}
	defer rows.Close()
	parts := make([]PartMeta, 0, hint)
	for rows.Next() {
		var p PartMeta
		if err := rows.Scan(&p.PartID, &p.EncryptedSize, &p.SHA256); err != nil {
			return nil, fmt.Errorf("scan part: %w", err)
		}
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

func (s *SQLiteState) storageKeysFor(ctx context.Context, sendIDs []string) ([]string, error) {
	placeholders, args := inClause(sendIDs)
	rows, err := s.db.QueryContext(ctx,
		`SELECT storage_key FROM send_parts WHERE send_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("storage keys: %w", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan storage key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func inClause(vals []string) (string, []any) {
	ph := make([]string, len(vals))
	args := make([]any, len(vals))
	for i, v := range vals {
		ph[i] = "?"
		args[i] = v
	}
	return strings.Join(ph, ","), args
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
