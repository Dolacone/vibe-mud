package authapi

import (
	"testing"
	"time"
)

func TestItemNormalizationScopesCleanupToRequesterAndCurrentLocation(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	requester, err := store.UpsertIdentity("https://accounts.google.com", "subject-normalization-requester", "requester@example.com", "Requester")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.UpsertIdentity("https://accounts.google.com", "subject-normalization-other", "other@example.com", "Other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
	INSERT INTO locations (id, display_name) VALUES ('remote-normalization', 'Remote Normalization')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES
		(?, 'wood', 'active', ?, 2),
		(?, 'wood_component', 'active', ?, 3)`, requester.ID, now.Add(-time.Second).Unix(), other.ID, now.Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO ground_items (location_id, item_id, durability_status, status_expires_at, quantity) VALUES
		('camp', 'wood', 'active', ?, 4),
		('remote-normalization', 'wood_component', 'active', ?, 5)`, now.Add(-time.Second).Unix(), now.Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}

	state, err := store.GetPlayerState(requester.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.ItemDurabilityCleanups) != 2 {
		t.Fatalf("requester cleanup events = %+v, want requester inventory and current ground only", state.ItemDurabilityCleanups)
	}
	for _, cleanup := range state.ItemDurabilityCleanups {
		if cleanup.ItemID == "wood_component" {
			t.Fatalf("requester cleanup included other player's item: %+v", cleanup)
		}
	}

	var status string
	if err := db.QueryRow(`SELECT durability_status FROM player_inventory WHERE user_id = ? AND item_id = 'wood_component'`, other.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("other player's inventory status = %q, want active", status)
	}
	if err := db.QueryRow(`SELECT durability_status FROM ground_items WHERE location_id = 'remote-normalization' AND item_id = 'wood_component'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("remote ground status = %q, want active", status)
	}
	if err := db.QueryRow(`SELECT durability_status FROM player_inventory WHERE user_id = ? AND item_id = 'wood'`, requester.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "expired" {
		t.Fatalf("requester inventory status = %q, want expired", status)
	}
	if err := db.QueryRow(`SELECT durability_status FROM ground_items WHERE location_id = 'camp' AND item_id = 'wood'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "expired" {
		t.Fatalf("current ground status = %q, want expired", status)
	}
}
