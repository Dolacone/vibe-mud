package authapi

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:auth-store-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(db)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return store, db
}

func TestUpsertIdentityKeepsTheSameUserAcrossProfileChanges(t *testing.T) {
	store, _ := newTestStore(t)

	first, err := store.UpsertIdentity("https://accounts.google.com", "subject-1", "old@example.com", "Old Name")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpsertIdentity("https://accounts.google.com", "subject-1", "new@example.com", "New Name")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != first.ID {
		t.Fatalf("profile changes created a second application user: first=%d updated=%d", first.ID, updated.ID)
	}
	if updated.Email != "new@example.com" || updated.DisplayName != "New Name" {
		t.Fatalf("mutable profile fields were not updated: %+v", updated)
	}
	if !updated.CreatedAt.Equal(first.CreatedAt) {
		t.Fatal("profile changes changed the identity creation time")
	}

	differentSubject, err := store.UpsertIdentity("https://accounts.google.com", "subject-2", "new@example.com", "New Name")
	if err != nil {
		t.Fatal(err)
	}
	if differentSubject.ID == first.ID {
		t.Fatal("different verified subjects were merged into one application user")
	}
}

func TestPlayerAPUsesOnlyFullTimestampAndHonorsMinuteBoundaries(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ap", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}

	ap, err := store.GetAP(identity.ID)
	if err != nil || ap != 3000 {
		t.Fatalf("new player AP = %d, %v; want 3000", ap, err)
	}
	var fullTimestamp int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp != now.UnixNano() {
		t.Fatalf("new player full timestamp = %d, want %d", fullTimestamp, now.UnixNano())
	}
	var columns []string
	rows, err := db.Query("PRAGMA table_info(player_ap)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0] != "user_id" || columns[1] != "full_timestamp" {
		t.Fatalf("player AP persisted columns = %v, want user_id and full_timestamp only", columns)
	}

	for _, test := range []struct {
		name   string
		fullAt time.Time
		wantAP int
	}{
		{name: "one minute", fullAt: now.Add(time.Minute), wantAP: 2999},
		{name: "just over one minute", fullAt: now.Add(time.Minute + time.Nanosecond), wantAP: 2998},
		{name: "at full boundary", fullAt: now.Add(3000 * time.Minute), wantAP: 0},
		{name: "past full boundary", fullAt: now.Add(-time.Nanosecond), wantAP: 3000},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", test.fullAt.UnixNano(), identity.ID); err != nil {
				t.Fatal(err)
			}
			got, err := store.GetAP(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.wantAP {
				t.Fatalf("AP = %d, want %d", got, test.wantAP)
			}
		})
	}
}

func TestRestAtomicallyConsumesAndPersistsAP(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-rest", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}

	ap, err := store.Rest(identity.ID)
	if err != nil || ap != 2999 {
		t.Fatalf("rest AP = %d, %v; want 2999", ap, err)
	}
	var fullTimestamp int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp != now.Add(time.Minute).UnixNano() {
		t.Fatalf("rest full timestamp = %d, want %d", fullTimestamp, now.Add(time.Minute).UnixNano())
	}

	reloaded, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = func() time.Time { return now }
	if ap, err := reloaded.GetAP(identity.ID); err != nil || ap != 2999 {
		t.Fatalf("persisted rest AP = %d, %v; want 2999", ap, err)
	}

	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(3000*time.Minute).UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}
	if ap, err := reloaded.Rest(identity.ID); !errors.Is(err, ErrInsufficientAP) || ap != 0 {
		t.Fatalf("rest without AP = %d, %v; want 0 and ErrInsufficientAP", ap, err)
	}
	if ap, err := reloaded.GetAP(identity.ID); err != nil || ap != 0 {
		t.Fatalf("AP after rejected rest = %d, %v; want 0", ap, err)
	}
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp != now.Add(3000*time.Minute).UnixNano() {
		t.Fatalf("rejected rest changed full timestamp to %d", fullTimestamp)
	}
}

func TestConsumeOAuthAttemptRecoversThenErasesSensitiveValues(t *testing.T) {
	store, db := newTestStore(t)
	expiresAt := time.Now().Add(time.Hour)
	if err := store.CreateOAuthAttempt("state-secret", "nonce-secret", "verifier-secret", expiresAt); err != nil {
		t.Fatal(err)
	}

	var storedState string
	if err := db.QueryRow("SELECT state_hash FROM oauth_attempts").Scan(&storedState); err != nil {
		t.Fatal(err)
	}
	if storedState == "state-secret" {
		t.Fatal("oauth state was persisted in plaintext")
	}

	attempt, err := store.ConsumeOAuthAttempt("state-secret")
	if err != nil {
		t.Fatal(err)
	}
	if attempt.Nonce != "nonce-secret" || attempt.Verifier != "verifier-secret" {
		t.Fatalf("callback could not recover PKCE values: %+v", attempt)
	}
	if !attempt.ExpiresAt.Equal(expiresAt.UTC().Truncate(time.Nanosecond)) {
		t.Fatalf("unexpected expiration: got %s want %s", attempt.ExpiresAt, expiresAt.UTC())
	}
	var nonce, verifier string
	var consumedAt sql.NullInt64
	if err := db.QueryRow("SELECT nonce, verifier, consumed_at FROM oauth_attempts WHERE state_hash = ?", hashSecret("state-secret")).Scan(&nonce, &verifier, &consumedAt); err != nil {
		t.Fatal(err)
	}
	if nonce != "" || verifier != "" || !consumedAt.Valid {
		t.Fatalf("consumption retained recoverable OAuth secrets: nonce=%q verifier=%q consumed=%v", nonce, verifier, consumedAt.Valid)
	}

	if _, err := store.ConsumeOAuthAttempt("state-secret"); !errors.Is(err, ErrOAuthAttemptConsumed) {
		t.Fatalf("replayed callback was accepted or returned the wrong error: %v", err)
	}
}

func TestConsumeOAuthAttemptRejectsExpiredState(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.CreateOAuthAttempt("expired-state", "nonce", "verifier", time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeOAuthAttempt("expired-state"); !errors.Is(err, ErrOAuthAttemptExpired) {
		t.Fatalf("expired callback was accepted or returned the wrong error: %v", err)
	}
}

func TestSessionLookupStoresOnlyAHashAndRejectsExpiry(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var storedToken string
	if err := db.QueryRow("SELECT token_hash FROM sessions").Scan(&storedToken); err != nil {
		t.Fatal(err)
	}
	if storedToken == "session-secret" {
		t.Fatal("session token was persisted in plaintext")
	}
	session, err := store.GetSession("session-secret")
	if err != nil {
		t.Fatal(err)
	}
	if session.UserID != identity.ID {
		t.Fatalf("session mapped to user %d, want %d", session.UserID, identity.ID)
	}

	if err := store.CreateSession(identity.ID, "expired-session", time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSession("expired-session"); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expired session was accepted or returned the wrong error: %v", err)
	}
}
