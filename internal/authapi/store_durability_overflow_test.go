package authapi

import (
	"math"
	"testing"
)

func TestActiveItemMergeUsesOverflowSafeFlooredDeadline(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-durability-overflow", "overflow@example.com", "Overflow")
	if err != nil {
		t.Fatal(err)
	}
	quantity := int64(math.MaxInt64 / 2)
	lowExpiry := int64(math.MaxInt64 - 1000)
	highExpiry := int64(math.MaxInt64 - 1)
	if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood', 'active', ?, ?)`, identity.ID, lowExpiry, quantity); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := addActiveItemHoldingTx(tx, "player_inventory", "user_id", identity.ID, "wood", int(quantity), highExpiry); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var mergedQuantity, mergedExpiry int64
	if err := db.QueryRow(`SELECT quantity, status_expires_at FROM player_inventory WHERE user_id = ? AND item_id = 'wood' AND durability_status = 'active'`, identity.ID).Scan(&mergedQuantity, &mergedExpiry); err != nil {
		t.Fatal(err)
	}
	wantQuantity := int64(math.MaxInt64 - 1)
	wantExpiry := int64(math.MaxInt64 - 501)
	if mergedQuantity != wantQuantity {
		t.Fatalf("merged quantity = %d, want %d", mergedQuantity, wantQuantity)
	}
	if mergedExpiry != wantExpiry {
		t.Fatalf("merged expiry = %d, want floored weighted deadline %d", mergedExpiry, wantExpiry)
	}
	if mergedExpiry < lowExpiry || mergedExpiry > highExpiry {
		t.Fatalf("merged expiry = %d, want value between %d and %d", mergedExpiry, lowExpiry, highExpiry)
	}
}

func TestWeightedActiveExpiryFloorsAcrossUnixEpoch(t *testing.T) {
	const (
		existingQuantity = int64(3)
		incomingQuantity = int64(2)
		existingExpiry   = int64(-5)
		incomingExpiry   = int64(4)
	)
	if got, want := weightedActiveExpiry(existingQuantity, incomingQuantity, existingExpiry, incomingExpiry), int64(-2); got != want {
		t.Fatalf("weighted expiry = %d, want floor %d", got, want)
	}
}
