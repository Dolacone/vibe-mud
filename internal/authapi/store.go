package authapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrIdentityNotFound     = errors.New("identity not found")
	ErrOAuthAttemptNotFound = errors.New("oauth attempt not found")
	ErrOAuthAttemptExpired  = errors.New("oauth attempt expired")
	ErrOAuthAttemptConsumed = errors.New("oauth attempt already consumed")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExpired       = errors.New("session expired")
	ErrInsufficientAP       = errors.New("insufficient action points")
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

type Identity struct {
	ID          int64
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OAuthAttempt struct {
	Nonce     string
	Verifier  string
	ExpiresAt time.Time
}

type Session struct {
	UserID    int64
	ExpiresAt time.Time
}

const (
	maxAP          = 3000
	apRecoveryTime = time.Minute
)

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database", ErrInvalidArgument)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		return nil, fmt.Errorf("configure auth store: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin auth store initialization: %w", err)
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issuer TEXT NOT NULL,
	subject TEXT NOT NULL,
	email TEXT NOT NULL,
	display_name TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	UNIQUE (issuer, subject)
);
	CREATE TABLE IF NOT EXISTS oauth_attempts (
	state_hash BLOB PRIMARY KEY,
	browser_token_hash BLOB,
	nonce TEXT NOT NULL,
	verifier TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	consumed_at INTEGER
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash BLOB PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES identities(id),
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS player_ap (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	full_timestamp INTEGER NOT NULL
);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("initialize auth store: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_ap (user_id, full_timestamp)
SELECT id, ? FROM identities`, time.Now().UTC().UnixNano()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player AP: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit auth store initialization: %w", err)
	}
	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) UpsertIdentity(issuer, subject, email, displayName string) (Identity, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(subject) == "" {
		return Identity{}, fmt.Errorf("%w: issuer and subject are required", ErrInvalidArgument)
	}
	now := s.now().UTC().UnixNano()
	tx, err := s.db.Begin()
	if err != nil {
		return Identity{}, fmt.Errorf("begin upsert identity: %w", err)
	}
	var identity Identity
	var createdAt, updatedAt int64
	err = tx.QueryRow(`
INSERT INTO identities (issuer, subject, email, display_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (issuer, subject) DO UPDATE SET
	email = excluded.email,
	display_name = excluded.display_name,
	updated_at = excluded.updated_at
RETURNING id, issuer, subject, email, display_name, created_at, updated_at`,
		issuer, subject, email, displayName, now, now,
	).Scan(&identity.ID, &identity.Issuer, &identity.Subject, &identity.Email, &identity.DisplayName, &createdAt, &updatedAt)
	if err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("upsert identity: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO player_ap (user_id, full_timestamp) VALUES (?, ?)
ON CONFLICT (user_id) DO NOTHING`, identity.ID, now); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player AP: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, fmt.Errorf("commit upsert identity: %w", err)
	}
	identity.CreatedAt = unixNano(createdAt)
	identity.UpdatedAt = unixNano(updatedAt)
	return identity, nil
}

func (s *Store) GetAP(userID int64) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	var fullTimestamp int64
	err := s.db.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrIdentityNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get player AP: %w", err)
	}
	return calculateAP(unixNano(fullTimestamp), s.now().UTC()), nil
}

func (s *Store) Rest(userID int64) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin rest: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return 0, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("get player AP for rest: %w", err)
	}
	now := s.now().UTC()
	if calculateAP(unixNano(fullTimestamp), now) == 0 {
		_ = tx.Rollback()
		return 0, ErrInsufficientAP
	}
	fullAt := unixNano(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(apRecoveryTime).UnixNano()
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, nextFullTimestamp, userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("rest player: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("check rest player: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return 0, ErrInsufficientAP
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit rest player: %w", err)
	}
	return calculateAP(unixNano(nextFullTimestamp), now), nil
}

func calculateAP(fullTimestamp, now time.Time) int {
	remaining := fullTimestamp.Sub(now)
	if remaining <= 0 {
		return maxAP
	}
	missing := remaining / apRecoveryTime
	if remaining%apRecoveryTime != 0 {
		missing++
	}
	if missing >= maxAP {
		return 0
	}
	return maxAP - int(missing)
}

func (s *Store) GetIdentity(id int64) (Identity, error) {
	var identity Identity
	var createdAt, updatedAt int64
	err := s.db.QueryRow(`
SELECT id, issuer, subject, email, display_name, created_at, updated_at
FROM identities WHERE id = ?`, id).Scan(
		&identity.ID, &identity.Issuer, &identity.Subject, &identity.Email,
		&identity.DisplayName, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrIdentityNotFound
	}
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}
	identity.CreatedAt = unixNano(createdAt)
	identity.UpdatedAt = unixNano(updatedAt)
	return identity, nil
}

func (s *Store) CreateOAuthAttempt(state, nonce, verifier string, expiresAt time.Time, browserToken ...string) error {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(nonce) == "" || strings.TrimSpace(verifier) == "" {
		return fmt.Errorf("%w: oauth attempt values are required", ErrInvalidArgument)
	}
	var browserHash any
	if len(browserToken) > 0 && strings.TrimSpace(browserToken[0]) != "" {
		browserHash = hashSecret(browserToken[0])
	}
	_, err := s.db.Exec(`
INSERT INTO oauth_attempts (state_hash, browser_token_hash, nonce, verifier, expires_at)
VALUES (?, ?, ?, ?, ?)`, hashSecret(state), browserHash, nonce, verifier, expiresAt.UTC().UnixNano())
	if err != nil {
		return fmt.Errorf("create oauth attempt: %w", err)
	}
	return nil
}

func (s *Store) ConsumeOAuthAttempt(state string, browserToken ...string) (OAuthAttempt, error) {
	if strings.TrimSpace(state) == "" {
		return OAuthAttempt{}, fmt.Errorf("%w: state is required", ErrInvalidArgument)
	}
	now := s.now().UTC()
	nowNanos := now.UnixNano()
	var browserHash any
	bound := len(browserToken) > 0
	boundFlag := 0
	if bound {
		browserHash = hashSecret(browserToken[0])
		boundFlag = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return OAuthAttempt{}, fmt.Errorf("begin oauth attempt consumption: %w", err)
	}
	var attempt OAuthAttempt
	var expiresAt int64
	err = tx.QueryRow(`
	SELECT nonce, verifier, expires_at FROM oauth_attempts
	WHERE state_hash = ? AND consumed_at IS NULL AND expires_at > ?
		AND (? = 0 OR browser_token_hash = ?)`, hashSecret(state), nowNanos, boundFlag, browserHash).Scan(
		&attempt.Nonce, &attempt.Verifier, &expiresAt,
	)
	if err == nil {
		result, updateErr := tx.Exec(`
UPDATE oauth_attempts
SET nonce = '', verifier = '', consumed_at = ?
WHERE state_hash = ? AND consumed_at IS NULL AND expires_at > ?
			AND (? = 0 OR browser_token_hash = ?)`, nowNanos, hashSecret(state), nowNanos, boundFlag, browserHash)
		if updateErr != nil {
			_ = tx.Rollback()
			return OAuthAttempt{}, fmt.Errorf("consume oauth attempt: %w", updateErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			_ = tx.Rollback()
			return OAuthAttempt{}, fmt.Errorf("check oauth attempt consumption: %w", rowsErr)
		}
		if rows != 1 {
			_ = tx.Rollback()
			return OAuthAttempt{}, ErrOAuthAttemptConsumed
		}
		if err = tx.Commit(); err != nil {
			return OAuthAttempt{}, fmt.Errorf("commit oauth attempt consumption: %w", err)
		}
		attempt.ExpiresAt = unixNano(expiresAt)
		return attempt, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return OAuthAttempt{}, fmt.Errorf("consume oauth attempt: %w", err)
	}
	var consumedAt sql.NullInt64
	err = tx.QueryRow(`
SELECT consumed_at, expires_at FROM oauth_attempts WHERE state_hash = ?`, hashSecret(state)).Scan(&consumedAt, &expiresAt)
	_ = tx.Rollback()
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthAttempt{}, ErrOAuthAttemptNotFound
	}
	if err != nil {
		return OAuthAttempt{}, fmt.Errorf("inspect oauth attempt: %w", err)
	}
	if consumedAt.Valid {
		return OAuthAttempt{}, ErrOAuthAttemptConsumed
	}
	if expiresAt <= nowNanos {
		return OAuthAttempt{}, ErrOAuthAttemptExpired
	}
	return OAuthAttempt{}, ErrOAuthAttemptNotFound
}

func (s *Store) CreateSession(userID int64, token string, expiresAt time.Time) error {
	if userID <= 0 || strings.TrimSpace(token) == "" {
		return fmt.Errorf("%w: user ID and token are required", ErrInvalidArgument)
	}
	_, err := s.db.Exec(`
INSERT INTO sessions (token_hash, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?)`, hashSecret(token), userID, expiresAt.UTC().UnixNano(), s.now().UTC().UnixNano())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) GetSession(token string) (Session, error) {
	if strings.TrimSpace(token) == "" {
		return Session{}, fmt.Errorf("%w: token is required", ErrInvalidArgument)
	}
	var session Session
	var expiresAt int64
	err := s.db.QueryRow(`
SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, hashSecret(token)).Scan(&session.UserID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	session.ExpiresAt = unixNano(expiresAt)
	if expiresAt <= s.now().UTC().UnixNano() {
		return Session{}, ErrSessionExpired
	}
	return session, nil
}

func (s *Store) GetIdentityForSession(token string) (Identity, error) {
	session, err := s.GetSession(token)
	if err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(session.UserID)
}

func hashSecret(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	encoded := make([]byte, hex.EncodedLen(len(digest)))
	hex.Encode(encoded, digest[:])
	return encoded
}

func unixNano(value int64) time.Time {
	return time.Unix(0, value).UTC()
}
