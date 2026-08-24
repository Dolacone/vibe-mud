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

func TestConsumeOAuthAttemptIsOneTimeAndRecoversPKCEValues(t *testing.T) {
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
