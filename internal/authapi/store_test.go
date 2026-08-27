package authapi

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
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

func resourceQuantity(state PlayerState, resourceID string) int {
	for _, resource := range state.Resources {
		if resource.Resource.ID == resourceID {
			return resource.Quantity
		}
	}
	return 0
}

func TestCraftLoadsSeededRecipeAndAtomicallyConsumesResources(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-craft", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 20)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.Craft(identity.ID, "wood_component")
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-10 || resourceQuantity(state, "wood") != 10 || len(state.Inventory) != 1 || state.Inventory[0].Item.ID != "wood_component" || state.Inventory[0].Quantity != 1 {
		t.Fatalf("craft state = %+v, want 10 Wood consumed and one output", state)
	}
	state, err = store.Craft(identity.ID, "wood_component")
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-20 || resourceQuantity(state, "wood") != 0 || state.Inventory[0].Quantity != 2 {
		t.Fatalf("second craft state = %+v, want accumulated output and zero Wood", state)
	}

	var recipeCount, resourceInputCount, itemInputCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM crafting_recipes WHERE id = 'wood_component'").Scan(&recipeCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM crafting_recipe_resource_inputs WHERE recipe_id = 'wood_component'").Scan(&resourceInputCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM crafting_recipe_item_inputs WHERE recipe_id = 'wood_component'").Scan(&itemInputCount); err != nil {
		t.Fatal(err)
	}
	if recipeCount != 1 || resourceInputCount != 1 || itemInputCount != 0 {
		t.Fatalf("seeded crafting schema = recipes %d, resources %d, items %d", recipeCount, resourceInputCount, itemInputCount)
	}
}

func TestCraftRejectsInvalidRecipesAndRollsBackEveryInput(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-craft-reject", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity) VALUES ('invalid', 'Invalid', 1, 'wood_component', 1)`); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, recipe := range state.CraftingRecipes {
		if recipe.ID == "invalid" {
			t.Fatal("recipe without Resource input was exposed")
		}
	}
	if _, err := store.Craft(identity.ID, "invalid"); !errors.Is(err, ErrCraftingNotFound) {
		t.Fatalf("invalid recipe error = %v, want ErrCraftingNotFound", err)
	}
	if _, err := store.Craft(identity.ID, "missing"); !errors.Is(err, ErrCraftingNotFound) {
		t.Fatalf("missing recipe error = %v, want ErrCraftingNotFound", err)
	}
	if _, err := store.Craft(identity.ID, "wood_component"); !errors.Is(err, ErrInsufficientResource) {
		t.Fatalf("missing resource error = %v, want ErrInsufficientResource", err)
	}
	state, err = store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP || len(state.Inventory) != 0 || resourceQuantity(state, "wood") != 0 {
		t.Fatalf("failed craft changed state = %+v", state)
	}
}

func TestCraftSupportsMultipleResourceAndOptionalItemInputs(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-craft-multi", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity) VALUES ('multi', 'Multi', 2, 'wood_component', 3);
INSERT INTO crafting_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES ('multi', 'wood', 2), ('multi', 'stone', 3);
INSERT INTO crafting_recipe_item_inputs (recipe_id, item_id, quantity) VALUES ('multi', 'wood', 1);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2), (?, 'stone', 3);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1);`, identity.ID, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.Craft(identity.ID, "multi")
	if err != nil {
		t.Fatal(err)
	}
	if resourceQuantity(state, "wood") != 0 || resourceQuantity(state, "stone") != 0 || len(state.Inventory) != 1 || state.Inventory[0].Item.ID != "wood_component" || state.Inventory[0].Quantity != 3 {
		t.Fatalf("multi-input craft state = %+v", state)
	}
}

func TestCraftingSchemaUpgradePreservesExistingPlayerState(t *testing.T) {
	db, err := sql.Open("sqlite", "file:crafting-schema-upgrade?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 27, 11, 0, 0, 0, time.UTC).UnixNano()
	if _, err := db.Exec(`
CREATE TABLE identities (id INTEGER PRIMARY KEY, issuer TEXT NOT NULL, subject TEXT NOT NULL, email TEXT NOT NULL, display_name TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE player_ap (user_id INTEGER PRIMARY KEY, full_timestamp INTEGER NOT NULL);
CREATE TABLE locations (id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
CREATE TABLE player_locations (user_id INTEGER PRIMARY KEY, location_id TEXT NOT NULL);
INSERT INTO identities VALUES (41, 'https://accounts.google.com', 'legacy-craft', 'person@example.com', 'Person', ?, ?);
INSERT INTO locations VALUES ('legacy-location', 'Legacy Location');
INSERT INTO player_ap VALUES (41, ?);
INSERT INTO player_locations VALUES (41, 'legacy-location');`, createdAt, createdAt, createdAt); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Unix(0, createdAt).UTC() }
	state, err := store.GetPlayerState(41)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "legacy-location" || state.AP != maxAP || len(state.CraftingRecipes) != 1 || state.CraftingRecipes[0].ID != "wood_component" {
		t.Fatalf("schema upgrade changed existing player state or omitted recipe: %+v", state)
	}
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

func TestNewStoreBackfillsPlayerAPForExistingIdentities(t *testing.T) {
	db, err := sql.Open("sqlite", "file:auth-store-upgrade-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
CREATE TABLE identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issuer TEXT NOT NULL,
	subject TEXT NOT NULL,
	email TEXT NOT NULL,
	display_name TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	UNIQUE (issuer, subject)
);
CREATE TABLE oauth_attempts (
	state_hash BLOB PRIMARY KEY,
	browser_token_hash BLOB,
	nonce TEXT NOT NULL,
	verifier TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	consumed_at INTEGER
);
CREATE TABLE sessions (
	token_hash BLOB PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES identities(id),
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE player_resources (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	balance INTEGER NOT NULL CHECK (balance >= 0)
);
CREATE TABLE locations (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL
);
CREATE TABLE items (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL
);
CREATE TABLE conversion_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	input_item_id TEXT NOT NULL REFERENCES items(id),
	input_quantity INTEGER NOT NULL CHECK (input_quantity > 0),
	resource_yield INTEGER NOT NULL CHECK (resource_yield > 0),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0)
);
INSERT INTO locations (id, display_name) VALUES ('camp', 'Camp');
INSERT INTO items (id, display_name) VALUES ('wood', 'Wood');
INSERT INTO conversion_rules (location_id, input_item_id, input_quantity, resource_yield, ap_cost)
VALUES ('camp', 'wood', 1, 1, 1);
INSERT INTO identities (issuer, subject, email, display_name, created_at, updated_at)
VALUES ('https://accounts.google.com', 'old-subject', 'old@example.com', 'Old Name', 1, 1);
INSERT INTO player_resources (user_id, balance) VALUES (1, 42);`); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	var userID int64
	if err := db.QueryRow("SELECT id FROM identities WHERE subject = ?", "old-subject").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var fullTimestamp int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", userID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp > time.Now().UTC().UnixNano() {
		t.Fatalf("backfilled full timestamp is in the future: %d", fullTimestamp)
	}
	backfillTime := unixNano(fullTimestamp)
	store.now = func() time.Time { return backfillTime }
	if ap, err := store.GetAP(userID); err != nil || ap != 3000 {
		t.Fatalf("backfilled identity AP = %d, %v; want 3000", ap, err)
	}
	var legacyBalanceColumn int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('player_resources') WHERE name = 'balance'").Scan(&legacyBalanceColumn); err != nil {
		t.Fatal(err)
	}
	if legacyBalanceColumn != 0 {
		t.Fatal("legacy generic Resource balance was retained")
	}
	state, err := store.GetPlayerState(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Resources) != 8 {
		t.Fatalf("backfilled identity Resources = %+v, want all eight types", state.Resources)
	}
	for _, resource := range state.Resources {
		if resource.Quantity != 0 {
			t.Fatalf("backfilled identity Resource %s = %d, want 0", resource.Resource.ID, resource.Quantity)
		}
	}
	var outputResourceID string
	if err := db.QueryRow("SELECT output_resource_id FROM conversion_rules WHERE location_id = 'camp'").Scan(&outputResourceID); err != nil {
		t.Fatal(err)
	}
	if outputResourceID != "wood" {
		t.Fatalf("migrated conversion output Resource = %q, want wood", outputResourceID)
	}

	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", time.Unix(0, 1).UnixNano(), userID); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", userID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp != 1 {
		t.Fatalf("existing player AP timestamp was overwritten: got %d, want 1", fullTimestamp)
	}
}

func TestRestConcurrentlyConsumesTheLastAPOnce(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-concurrent-rest", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(2999*time.Minute).UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	type restResult struct {
		ap  int
		err error
	}
	results := make(chan restResult, 2)
	for range 2 {
		go func() {
			ready <- struct{}{}
			<-start
			ap, err := store.Rest(identity.ID)
			results <- restResult{ap: ap, err: err}
		}()
	}
	<-ready
	<-ready
	close(start)

	successes := 0
	insufficient := 0
	for range 2 {
		result := <-results
		if result.err == nil {
			successes++
			if result.ap != 0 {
				t.Fatalf("successful concurrent rest returned AP %d, want 0", result.ap)
			}
		} else if errors.Is(result.err, ErrInsufficientAP) {
			insufficient++
		} else {
			t.Fatalf("concurrent rest returned unexpected error: %v", result.err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent rest outcomes = %d successes, %d insufficient; want one each", successes, insufficient)
	}
	if ap, err := store.GetAP(identity.ID); err != nil || ap != 0 {
		t.Fatalf("persisted AP after concurrent rest = %d, %v; want 0", ap, err)
	}
}

func TestPlayerStateStartsAtCampWithSeededRoutes(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-movement", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}

	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.Location.DisplayName != "Camp" {
		t.Fatalf("new player location = %+v, want camp", state.Location)
	}
	if len(state.Routes) != 1 || state.Routes[0] != (Route{OriginID: "camp", DestinationID: "forest_edge", APCost: 20}) {
		t.Fatalf("new player routes = %+v, want camp to forest_edge at 20 AP", state.Routes)
	}
	if state.AP != maxAP {
		t.Fatalf("new player AP = %d, want %d", state.AP, maxAP)
	}
	if len(state.Inventory) != 0 {
		t.Fatalf("new player inventory = %+v, want empty inventory", state.Inventory)
	}
	if state.GatheringOption != nil {
		t.Fatalf("camp gathering option = %+v, want none", state.GatheringOption)
	}
	if state.ConversionOption == nil || state.ConversionOption.Item.ID != "wood" || state.ConversionOption.Item.DisplayName != "Wood" || state.ConversionOption.Resource.ID != "wood" || state.ConversionOption.Resource.DisplayName != "Wood" || state.ConversionOption.InputQuantity != 1 || state.ConversionOption.ResourceYield != 1 || state.ConversionOption.APCost != 1 {
		t.Fatalf("camp conversion option = %+v, want backend seed", state.ConversionOption)
	}
	wantResources := []string{"arcane", "fiber", "food", "hide", "medicinal", "metal", "stone", "wood"}
	if len(state.Resources) != len(wantResources) {
		t.Fatalf("new player Resources = %+v, want eight resources", state.Resources)
	}
	for index, resource := range state.Resources {
		if resource.Resource.ID != wantResources[index] || resource.Quantity != 0 {
			t.Fatalf("new player Resource[%d] = %+v, want %s at zero", index, resource, wantResources[index])
		}
	}
}

func TestTypedResourceRowsAreIndependentAndMissingRowsReadAsZero(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-typed-resource", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'stone', 7), (?, 'wood', 11)`, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	quantities := make(map[string]int, len(state.Resources))
	for _, resource := range state.Resources {
		quantities[resource.Resource.ID] = resource.Quantity
	}
	if quantities["wood"] != 11 || quantities["stone"] != 7 || quantities["food"] != 0 || len(quantities) != 8 {
		t.Fatalf("typed Resource quantities = %v, want independent values and zero missing rows", quantities)
	}
	var rows int
	if err := db.QueryRow("SELECT COUNT(*) FROM player_resources WHERE user_id = ?", identity.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("stored typed Resource rows = %d, want only explicitly held types", rows)
	}
}

func TestTypedResourceSchemaIsStableAcrossRepeatedStoreInitialization(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-typed-resource-reload", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'metal', 23)", identity.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		store, err = NewStore(db)
		if err != nil {
			t.Fatal(err)
		}
		store.now = func() time.Time { return now }
		state, err := store.GetPlayerState(identity.ID)
		if err != nil {
			t.Fatal(err)
		}
		var metal int
		for _, resource := range state.Resources {
			if resource.Resource.ID == "metal" {
				metal = resource.Quantity
			}
		}
		if metal != 23 {
			t.Fatalf("reinitialized Metal Resource = %d, want persisted 23", metal)
		}
	}
}

func TestGatherConsumesAPAndAccumulatesInventory(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}

	state, err := store.Gather(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-30 {
		t.Fatalf("gather AP = %d, want %d", state.AP, maxAP-30)
	}
	if state.Location.ID != "forest_edge" {
		t.Fatalf("gather location = %s, want forest_edge", state.Location.ID)
	}
	if state.GatheringOption == nil || state.GatheringOption.Item.ID != "wood" || state.GatheringOption.Item.DisplayName != "Wood" || state.GatheringOption.Quantity != 1 || state.GatheringOption.APCost != 10 {
		t.Fatalf("gathering option = %+v, want backend seed", state.GatheringOption)
	}
	if len(state.Inventory) != 1 || state.Inventory[0].Item.ID != "wood" || state.Inventory[0].Item.DisplayName != "Wood" || state.Inventory[0].Quantity != 1 {
		t.Fatalf("gather inventory = %+v, want one Wood", state.Inventory)
	}

	state, err = store.Gather(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-40 || len(state.Inventory) != 1 || state.Inventory[0].Quantity != 2 {
		t.Fatalf("repeated gather state = %+v, want AP %d and Wood quantity 2", state, maxAP-40)
	}
}

func TestGatherPersistsInventoryAcrossStoreReload(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather-persist", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Gather(identity.ID); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = func() time.Time { return now }
	state, err := reloaded.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Inventory) != 1 || state.Inventory[0].Item.ID != "wood" || state.Inventory[0].Quantity != 1 {
		t.Fatalf("reloaded inventory = %+v, want one persisted Wood", state.Inventory)
	}
}

func TestGatherRejectsInvalidLocationWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather-location", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Gather(identity.ID); !errors.Is(err, ErrGatheringNotFound) {
		t.Fatalf("camp gather error = %v, want ErrGatheringNotFound", err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.AP != maxAP || len(state.Inventory) != 0 {
		t.Fatalf("state after invalid-location gather = %+v, want unchanged camp state", state)
	}
	var after int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("invalid-location gather changed AP timestamp from %d to %d", before, after)
	}
}

func TestGatherRejectsInsufficientAPWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather-insufficient", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	before := now.Add(maxAP * time.Minute).UnixNano()
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", before, identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Gather(identity.ID); !errors.Is(err, ErrInsufficientAP) {
		t.Fatalf("gather with insufficient AP error = %v, want ErrInsufficientAP", err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "forest_edge" || state.AP != 0 || len(state.Inventory) != 0 {
		t.Fatalf("state after insufficient gather = %+v, want unchanged forest state", state)
	}
	var after int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("insufficient gather changed AP timestamp from %d to %d", before, after)
	}
}

func TestGatherRollsBackAPWhenInventoryWriteFails(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather-rollback", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_gather_inventory_insert
BEFORE INSERT ON player_inventory
BEGIN
SELECT RAISE(ABORT, 'test inventory failure');
END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Gather(identity.ID); err == nil {
		t.Fatal("gather succeeded despite inventory write failure")
	}
	var after int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("rolled-back gather changed AP timestamp from %d to %d", before, after)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM player_inventory WHERE user_id = ?", identity.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled-back gather persisted %d inventory rows", count)
	}
}

func TestGatherConcurrentlyConsumesTheLastGatherAPOnce(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather-concurrent", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add((maxAP-10)*time.Minute).UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	type gatherResult struct {
		state PlayerState
		err   error
	}
	results := make(chan gatherResult, 2)
	for range 2 {
		go func() {
			ready <- struct{}{}
			<-start
			state, err := store.Gather(identity.ID)
			results <- gatherResult{state: state, err: err}
		}()
	}
	<-ready
	<-ready
	close(start)

	successes := 0
	insufficient := 0
	for range 2 {
		result := <-results
		if result.err == nil {
			successes++
			if result.state.AP != 0 || len(result.state.Inventory) != 1 || result.state.Inventory[0].Quantity != 1 {
				t.Fatalf("successful concurrent gather state = %+v, want zero AP and one Wood", result.state)
			}
		} else if errors.Is(result.err, ErrInsufficientAP) {
			insufficient++
		} else {
			t.Fatalf("concurrent gather returned unexpected error: %v", result.err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent gather outcomes = %d successes, %d insufficient; want one each", successes, insufficient)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != 0 || len(state.Inventory) != 1 || state.Inventory[0].Quantity != 1 {
		t.Fatalf("persisted concurrent gather state = %+v, want zero AP and one Wood", state)
	}
}

func TestPlayerStateReadsOneSQLiteSnapshotDuringConcurrentMove(t *testing.T) {
	const hookName = "test_player_state_snapshot_hook"
	var writer *sql.DB
	moveStarted := make(chan struct{})
	moveCommitted := make(chan struct{})
	moveErrors := make(chan error, 1)
	var startOnce sync.Once
	sqlite.MustRegisterScalarFunction(hookName, 0, func(_ *sqlite.FunctionContext, _ []driver.Value) (driver.Value, error) {
		startOnce.Do(func() {
			close(moveStarted)
			go func() {
				tx, err := writer.Begin()
				if err == nil {
					_, err = tx.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = 1", time.Date(2026, 8, 25, 12, 20, 0, 0, time.UTC).UnixNano())
				}
				if err == nil {
					_, err = tx.Exec("UPDATE player_locations SET location_id = 'forest_edge' WHERE user_id = 1")
				}
				if err == nil {
					err = tx.Commit()
				} else if tx != nil {
					_ = tx.Rollback()
				}
				moveErrors <- err
				close(moveCommitted)
			}()
		})
		select {
		case <-moveCommitted:
		case <-time.After(100 * time.Millisecond):
		}
		return int64(1), nil
	})

	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-snapshot", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ID != 1 {
		t.Fatalf("test identity ID = %d, want 1", identity.ID)
	}

	writer, err = sql.Open("sqlite", "file:auth-store-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	if _, err := writer.Exec("PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
ALTER TABLE routes RENAME TO routes_snapshot_base;
CREATE VIEW routes AS
SELECT origin_id, destination_id, ap_cost
FROM routes_snapshot_base
WHERE test_player_state_snapshot_hook() = 1;`); err != nil {
		t.Fatal(err)
	}

	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-moveErrors:
		if err != nil {
			t.Fatalf("concurrent move failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent move did not complete")
	}
	if state.Location.ID != "camp" || state.AP != maxAP || len(state.Routes) != 1 || state.Routes[0].DestinationID != "forest_edge" {
		t.Fatalf("player state combined different SQLite snapshots: %+v", state)
	}
}

func TestMoveAtomicallyConsumesAPAndPersistsLocation(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-movement-success", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}

	state, err := store.Move(identity.ID, "forest_edge")
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "forest_edge" || state.AP != maxAP-20 {
		t.Fatalf("move state = %+v, want forest_edge and AP %d", state, maxAP-20)
	}
	if len(state.Routes) != 1 || state.Routes[0].DestinationID != "camp" || state.Routes[0].APCost != 20 {
		t.Fatalf("move routes = %+v, want return route to camp at 20 AP", state.Routes)
	}

	reloaded, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = func() time.Time { return now }
	persisted, err := reloaded.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Location.ID != "forest_edge" || persisted.AP != maxAP-20 {
		t.Fatalf("persisted move state = %+v, want forest_edge and AP %d", persisted, maxAP-20)
	}
}

func TestMoveRejectsInsufficientAPWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-movement-insufficient", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	before := now.Add(maxAP * time.Minute).UnixNano()
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", before, identity.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Move(identity.ID, "forest_edge"); !errors.Is(err, ErrInsufficientAP) {
		t.Fatalf("move with insufficient AP error = %v, want ErrInsufficientAP", err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.AP != 0 {
		t.Fatalf("state after rejected move = %+v, want camp and AP 0", state)
	}
	var fullTimestamp int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp != before {
		t.Fatalf("rejected move changed full timestamp to %d, want %d", fullTimestamp, before)
	}
}

func TestMoveRejectsUnavailableRouteWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-movement-invalid", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	before := now.UnixNano()
	if _, err := store.Move(identity.ID, "unknown"); !errors.Is(err, ErrRouteNotFound) {
		t.Fatalf("move with unavailable route error = %v, want ErrRouteNotFound", err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.AP != maxAP {
		t.Fatalf("state after rejected route = %+v, want camp and AP %d", state, maxAP)
	}
	var fullTimestamp int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&fullTimestamp); err != nil {
		t.Fatal(err)
	}
	if fullTimestamp != before {
		t.Fatalf("rejected route changed full timestamp to %d, want %d", fullTimestamp, before)
	}
}

func TestConvertConsumesWoodAPAndAccumulatesResource(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-success", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 2)", identity.ID); err != nil {
		t.Fatal(err)
	}

	state, err := store.Convert(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-1 || len(state.Resources) != 8 || resourceQuantity(state, "wood") != 1 || len(state.Inventory) != 1 || state.Inventory[0].Item.ID != "wood" || state.Inventory[0].Quantity != 1 {
		t.Fatalf("first convert state = %+v, want one Wood, one Resource and AP %d", state, maxAP-1)
	}
	state, err = store.Convert(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-2 || resourceQuantity(state, "wood") != 2 || len(state.Inventory) != 0 {
		t.Fatalf("second convert state = %+v, want empty inventory, two Resources and AP %d", state, maxAP-2)
	}

	reloaded, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = func() time.Time { return now }
	persisted, err := reloaded.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resourceQuantity(persisted, "wood") != 2 || len(persisted.Inventory) != 0 || persisted.AP != maxAP-2 {
		t.Fatalf("reloaded convert state = %+v, want persisted Resource and AP", persisted)
	}
}

func TestConvertRejectsWrongLocationWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-location", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)", identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Convert(identity.ID); !errors.Is(err, ErrConversionNotFound) {
		t.Fatalf("convert outside camp error = %v, want ErrConversionNotFound", err)
	}
	after, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Location != before.Location || after.AP != before.AP || len(after.Inventory) != 1 || after.Inventory[0].Quantity != before.Inventory[0].Quantity || resourceQuantity(after, "wood") != resourceQuantity(before, "wood") {
		t.Fatalf("state after wrong-location convert = %+v, want unchanged %+v", after, before)
	}
}

func TestConvertRejectsMissingWoodWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-item", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Convert(identity.ID); !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("convert without Wood error = %v, want ErrInsufficientItem", err)
	}
	var after int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("missing-Wood convert changed AP timestamp from %d to %d", before, after)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP || len(state.Inventory) != 0 || resourceQuantity(state, "wood") != 0 {
		t.Fatalf("state after missing-Wood convert = %+v, want unchanged", state)
	}
}

func TestConvertRejectsInsufficientAPWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-ap", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)", identity.ID); err != nil {
		t.Fatal(err)
	}
	before := now.Add(maxAP * time.Minute).UnixNano()
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", before, identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Convert(identity.ID); !errors.Is(err, ErrInsufficientAP) {
		t.Fatalf("convert with insufficient AP error = %v, want ErrInsufficientAP", err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != 0 || len(state.Inventory) != 1 || state.Inventory[0].Quantity != 1 || resourceQuantity(state, "wood") != 0 {
		t.Fatalf("state after insufficient-AP convert = %+v, want unchanged", state)
	}
	var after int64
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("insufficient-AP convert changed timestamp from %d to %d", before, after)
	}
}

func TestConvertRollsBackAPAndWoodWhenResourceWriteFails(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-rollback", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)", identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_convert_resource_insert
BEFORE INSERT ON player_resources
BEGIN
SELECT RAISE(ABORT, 'test resource failure');
END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Convert(identity.ID); err == nil {
		t.Fatal("convert succeeded despite resource write failure")
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP || len(state.Inventory) != 1 || state.Inventory[0].Quantity != 1 || resourceQuantity(state, "wood") != 0 {
		t.Fatalf("rolled-back convert state = %+v, want original state", state)
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
