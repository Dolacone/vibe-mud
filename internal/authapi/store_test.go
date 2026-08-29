package authapi

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
)

const (
	expectedBuildingDurabilitySeconds   int64 = 7 * 24 * 60 * 60
	expectedItemDurabilitySeconds       int64 = 60 * 60
	expectedItemExpiredRetentionSeconds int64 = 24 * 60 * 60
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

func inventoryQuantity(state PlayerState, itemID string) int {
	for _, item := range state.Inventory {
		if item.Item.ID == itemID {
			return item.Quantity
		}
	}
	return 0
}

func TestBuildAtomicallyConsumesInputsAndSnapshotsRecipe(t *testing.T) {
	store, db := newTestStore(t)
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-build-start", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO building_recipes (id, display_name, building_level, required_ap, extension_slot_count) VALUES ('resource_building', 'Resource Building', 2, 45, 3);
INSERT INTO building_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES ('resource_building', 'wood', 2), ('resource_building', 'stone', 3);
INSERT INTO building_recipe_item_inputs (recipe_id, item_id, quantity) VALUES ('resource_building', 'wood_component', 1);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2), (?, 'stone', 5);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1);`, owner.ID, owner.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.Build(owner.ID, "resource_building")
	if err != nil {
		t.Fatal(err)
	}
	if resourceQuantity(state, "wood") != 0 || resourceQuantity(state, "stone") != 2 || inventoryQuantity(state, "wood_component") != 0 {
		t.Fatalf("build inputs = %+v, want exact resource and item depletion", state)
	}
	if len(state.Buildings) != 1 {
		t.Fatalf("buildings = %+v, want one building", state.Buildings)
	}
	building := state.Buildings[0]
	if building.Owner.ID != owner.ID || building.Recipe.ID != "resource_building" || building.Recipe.DisplayName != "Resource Building" || building.BuildingLevel != 2 || building.RequiredAP != 45 || building.ContributedAP != 0 || building.Status != "under_construction" || building.ExtensionSlotCount != 3 {
		t.Fatalf("building snapshot = %+v", building)
	}
	if _, err := db.Exec(`UPDATE building_recipes SET display_name = 'Changed', building_level = 9, required_ap = 99, extension_slot_count = 0 WHERE id = 'resource_building'`); err != nil {
		t.Fatal(err)
	}
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	building = state.Buildings[0]
	if building.Recipe.DisplayName != "Resource Building" || building.BuildingLevel != 2 || building.RequiredAP != 45 || building.ExtensionSlotCount != 3 {
		t.Fatalf("building snapshot changed after recipe update = %+v", building)
	}
}

func TestBuildSupportsItemOnlyRecipeAndEnforcesLocationUniqueness(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-build-unique", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO building_recipes (id, display_name, building_level, required_ap, extension_slot_count) VALUES ('item_building', 'Item Building', 1, 10, 1);
INSERT INTO building_recipe_item_inputs (recipe_id, item_id, quantity) VALUES ('item_building', 'wood_component', 1);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 2);`, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(owner.ID, "item_building"); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(owner.ID, "item_building"); !errors.Is(err, ErrBuildingOccupied) {
		t.Fatalf("duplicate building error = %v, want ErrBuildingOccupied", err)
	}
	after, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("duplicate build changed state: before=%+v after=%+v", before, after)
	}
}

func TestBuildRejectsUnknownOrInsufficientInputsWithoutChangingState(t *testing.T) {
	store, db := newTestStore(t)
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-build-rollback", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO building_recipes (id, display_name, building_level, required_ap, extension_slot_count) VALUES ('mixed_building', 'Mixed Building', 1, 20, 1);
INSERT INTO building_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES ('mixed_building', 'wood', 2), ('mixed_building', 'stone', 1);
INSERT INTO building_recipe_item_inputs (recipe_id, item_id, quantity) VALUES ('mixed_building', 'wood_component', 1);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1);`, owner.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(owner.ID, "unknown"); !errors.Is(err, ErrBuildingNotFound) {
		t.Fatalf("unknown recipe error = %v, want ErrBuildingNotFound", err)
	}
	if _, err := store.Build(owner.ID, "mixed_building"); !errors.Is(err, ErrInsufficientResource) {
		t.Fatalf("insufficient resource error = %v, want ErrInsufficientResource", err)
	}
	after, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed build changed state: before=%+v after=%+v", before, after)
	}
}

func TestContributeConstructionSharesAPAndCompletesWithOversizedRequest(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	contributor, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-contributor", "contributor@example.com", "Contributor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (?, 'camp', 'building_lv1', 1, 60, 20, 'under_construction', 1)`, owner.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.ContributeConstruction(contributor.ID, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-40 || len(state.Buildings) != 1 || state.Buildings[0].ContributedAP != 60 || state.Buildings[0].Status != "completed" {
		t.Fatalf("contribution state = %+v, want capped completion and 40 AP spent", state)
	}
	before, err := store.GetPlayerState(contributor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ContributeConstruction(contributor.ID, 1, 1); !errors.Is(err, ErrBuildingCompleted) {
		t.Fatalf("repeated completion error = %v, want ErrBuildingCompleted", err)
	}
	after, err := store.GetPlayerState(contributor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("repeated completion changed state: before=%+v after=%+v", before, after)
	}
}

func TestContributeConstructionRejectsInsufficientAPAndRemoteTargetWithoutRollback(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-rollback-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	contributor, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-rollback-contributor", "contributor@example.com", "Contributor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (?, 'camp',  'building_lv1', 1, 60, 0, 'under_construction', 1)`, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, now.Add(2995*time.Minute).Unix(), contributor.ID); err != nil {
		t.Fatal(err)
	}
	if ap, err := store.GetAP(contributor.ID); err != nil || ap != 5 {
		t.Fatalf("configured contributor AP = %d, %v; want 5", ap, err)
	}
	before, err := store.GetPlayerState(contributor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ContributeConstruction(contributor.ID, 10, 10); !errors.Is(err, ErrBuildingNotFound) {
		t.Fatalf("unknown target error = %v, want ErrBuildingNotFound", err)
	}
	if _, err := store.ContributeConstruction(contributor.ID, 1, 10); !errors.Is(err, ErrInsufficientAP) {
		t.Fatalf("insufficient AP error = %v, want ErrInsufficientAP", err)
	}
	after, err := store.GetPlayerState(contributor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("insufficient AP changed state: before=%+v after=%+v", before, after)
	}
	if _, err := db.Exec(`INSERT INTO locations (id, display_name) VALUES ('remote', 'Remote')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE player_locations SET location_id = 'remote' WHERE user_id = ?`, contributor.ID); err != nil {
		t.Fatal(err)
	}
	before, err = store.GetPlayerState(contributor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ContributeConstruction(contributor.ID, 1, 1); !errors.Is(err, ErrBuildingRemote) {
		t.Fatalf("remote target error = %v, want ErrBuildingRemote", err)
	}
	after, err = store.GetPlayerState(contributor.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("remote contribution changed state: before=%+v after=%+v", before, after)
	}
}

func TestItemDefinitionsUseOneHourDurability(t *testing.T) {
	_, db := newTestStore(t)
	rows, err := db.Query(`SELECT id, weight_units, max_durability_seconds FROM items ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wantWeights := map[string]int{"wood": 100, "wood_component": 10, "wood_essence_t1": 1, "sawmill_package_t1": 10}
	for rows.Next() {
		var id string
		var weightUnits, maxDurabilitySeconds int
		if err := rows.Scan(&id, &weightUnits, &maxDurabilitySeconds); err != nil {
			t.Fatal(err)
		}
		if maxDurabilitySeconds != int(time.Hour/time.Second) {
			t.Fatalf("item %q durability = %d, want one hour", id, maxDurabilitySeconds)
		}
		if weightUnits != wantWeights[id] {
			t.Fatalf("item %q weight = %d, want %d", id, weightUnits, wantWeights[id])
		}
		delete(wantWeights, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(wantWeights) != 0 {
		t.Fatalf("missing item definitions: %v", wantWeights)
	}
}

func TestSawmillDefinitionsExposeTypedBalanceValuesToPlayerState(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-sawmill-definitions", "sawmill@example.com", "Sawmill Player")
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.ConversionMethods) != 2 {
		t.Fatalf("conversion methods = %+v, want hand and sawmill methods", state.ConversionMethods)
	}
	methods := make(map[string]ConversionMethod, len(state.ConversionMethods))
	for _, method := range state.ConversionMethods {
		methods[method.ID] = method
	}
	hand := methods["hand_wood_t1"]
	if hand.DisplayName != "Hand Wood Convert" || hand.APCost != 30 || hand.Input.ID != "wood" || hand.MaxInputQuantity != 3 || hand.OutputResource.ID != "wood" || hand.ResourceQuantityPerInput != 1 || hand.EssenceItem == nil || hand.EssenceItem.ID != "wood_essence_t1" || hand.EssenceChanceBPS != 1000 || hand.EssenceQuantity != 1 {
		t.Fatalf("hand conversion definition = %+v", hand)
	}
	sawmill := methods["sawmill_wood_t1"]
	if sawmill.MaxInputQuantity != 6 || sawmill.EssenceItem == nil || sawmill.EssenceItem.ID != "wood_essence_t1" {
		t.Fatalf("sawmill conversion definition = %+v", sawmill)
	}
	if len(state.BuildingExtensionDefinitions) != 1 {
		t.Fatalf("extension definitions = %+v, want Sawmill T1", state.BuildingExtensionDefinitions)
	}
	definition := state.BuildingExtensionDefinitions[0]
	if definition.ID != "sawmill_t1" || definition.DisplayName != "Sawmill T1" || definition.Tier != 1 || definition.PackageItem.ID != "sawmill_package_t1" || definition.RequiredAP != 30 {
		t.Fatalf("sawmill extension definition = %+v", definition)
	}
	var resourceInputs, itemInputs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM crafting_recipe_resource_inputs WHERE recipe_id = 'sawmill_package_t1'`).Scan(&resourceInputs); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM crafting_recipe_item_inputs WHERE recipe_id = 'sawmill_package_t1'`).Scan(&itemInputs); err != nil {
		t.Fatal(err)
	}
	if resourceInputs != 1 || itemInputs != 1 {
		t.Fatalf("sawmill package inputs = resources %d, items %d; want one each", resourceInputs, itemInputs)
	}
	var recipeAP int
	var recipeOutput string
	var recipeOutputQuantity int
	if err := db.QueryRow(`SELECT base_ap_cost, output_item_id, output_quantity FROM crafting_recipes WHERE id = 'sawmill_package_t1'`).Scan(&recipeAP, &recipeOutput, &recipeOutputQuantity); err != nil {
		t.Fatal(err)
	}
	if recipeAP != 30 || recipeOutput != "sawmill_package_t1" || recipeOutputQuantity != 1 {
		t.Fatalf("sawmill package recipe = AP %d, output %q x%d", recipeAP, recipeOutput, recipeOutputQuantity)
	}
	var resourceID, itemID string
	var resourceQuantity, itemQuantity int
	if err := db.QueryRow(`SELECT resource_id, quantity FROM crafting_recipe_resource_inputs WHERE recipe_id = 'sawmill_package_t1'`).Scan(&resourceID, &resourceQuantity); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT item_id, quantity FROM crafting_recipe_item_inputs WHERE recipe_id = 'sawmill_package_t1'`).Scan(&itemID, &itemQuantity); err != nil {
		t.Fatal(err)
	}
	if resourceID != "wood" || resourceQuantity != 10 || itemID != "wood_essence_t1" || itemQuantity != 1 {
		t.Fatalf("sawmill package inputs = resource %q x%d, item %q x%d", resourceID, resourceQuantity, itemID, itemQuantity)
	}
	var durabilityCost int
	if err := db.QueryRow(`SELECT building_durability_cost_seconds FROM extension_conversion_capabilities WHERE extension_definition_id = 'sawmill_t1' AND conversion_method_id = 'sawmill_wood_t1'`).Scan(&durabilityCost); err != nil {
		t.Fatal(err)
	}
	if durabilityCost != 60 {
		t.Fatalf("sawmill durability cost = %d, want 60", durabilityCost)
	}
}

func TestStoreReinitializationPreservesDirectBalanceEdits(t *testing.T) {
	_, db := newTestStore(t)
	if _, err := db.Exec(`
UPDATE items SET display_name = 'Edited Wood', weight_units = 77, max_durability_seconds = 1234 WHERE id = 'wood';
UPDATE resource_types SET display_name = 'Edited Wood Resource', weight_units = 8 WHERE id = 'wood';
UPDATE conversion_methods SET display_name = 'Edited Hand Convert', ap_cost = 17, max_input_quantity = 2 WHERE id = 'hand_wood_t1';
UPDATE building_extension_definitions SET display_name = 'Edited Sawmill', required_ap = 44 WHERE id = 'sawmill_t1';
UPDATE crafting_recipes SET display_name = 'Edited Package', base_ap_cost = 31 WHERE id = 'sawmill_package_t1';`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(db); err != nil {
		t.Fatal(err)
	}
	var itemName string
	var itemWeight, itemDurability int
	if err := db.QueryRow(`SELECT display_name, weight_units, max_durability_seconds FROM items WHERE id = 'wood'`).Scan(&itemName, &itemWeight, &itemDurability); err != nil {
		t.Fatal(err)
	}
	if itemName != "Edited Wood" || itemWeight != 77 || itemDurability != 1234 {
		t.Fatalf("direct item edit after reinitialization = %q, %d, %d", itemName, itemWeight, itemDurability)
	}
	var resourceName string
	var resourceWeight int
	if err := db.QueryRow(`SELECT display_name, weight_units FROM resource_types WHERE id = 'wood'`).Scan(&resourceName, &resourceWeight); err != nil {
		t.Fatal(err)
	}
	if resourceName != "Edited Wood Resource" || resourceWeight != 8 {
		t.Fatalf("direct resource edit after reinitialization = %q, %d", resourceName, resourceWeight)
	}
	var methodName string
	var methodAP, methodCapacity int
	if err := db.QueryRow(`SELECT display_name, ap_cost, max_input_quantity FROM conversion_methods WHERE id = 'hand_wood_t1'`).Scan(&methodName, &methodAP, &methodCapacity); err != nil {
		t.Fatal(err)
	}
	if methodName != "Edited Hand Convert" || methodAP != 17 || methodCapacity != 2 {
		t.Fatalf("direct conversion edit after reinitialization = %q, %d, %d", methodName, methodAP, methodCapacity)
	}
	var extensionName string
	var extensionAP int
	if err := db.QueryRow(`SELECT display_name, required_ap FROM building_extension_definitions WHERE id = 'sawmill_t1'`).Scan(&extensionName, &extensionAP); err != nil {
		t.Fatal(err)
	}
	if extensionName != "Edited Sawmill" || extensionAP != 44 {
		t.Fatalf("direct extension edit after reinitialization = %q, %d", extensionName, extensionAP)
	}
}

func TestStoreReinitializationSeedsOnlyMissingBalanceRows(t *testing.T) {
	_, db := newTestStore(t)
	if _, err := db.Exec(`DELETE FROM global_conversion_methods WHERE conversion_method_id = 'hand_wood_t1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM global_conversion_methods WHERE conversion_method_id = 'hand_wood_t1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("missing global method seed count = %d, want one", count)
	}
}

func TestContributeConstructionSerializesConcurrentSharedContributors(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-concurrent-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-concurrent-first", "first@example.com", "First")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.UpsertIdentity("https://accounts.google.com", "subject-construction-concurrent-second", "second@example.com", "Second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (?, 'camp', 'building_lv1', 1, 60, 0, 'under_construction', 1)`, owner.ID); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, userID := range []int64{first.ID, second.ID} {
		go func(userID int64) {
			<-start
			_, err := store.ContributeConstruction(userID, 1, 60)
			results <- err
		}(userID)
	}
	close(start)
	var successes, completed int
	for range 2 {
		switch err := <-results; {
		case err == nil:
			successes++
		case errors.Is(err, ErrBuildingCompleted):
			completed++
		default:
			t.Fatalf("concurrent contribution error = %v", err)
		}
	}
	if successes != 1 || completed != 1 {
		t.Fatalf("concurrent outcomes = %d successes, %d completed; want one each", successes, completed)
	}
	state, err := store.GetPlayerState(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 1 || state.Buildings[0].ContributedAP != 60 || state.Buildings[0].Status != "completed" {
		t.Fatalf("persisted concurrent construction = %+v, want exact completion", state.Buildings)
	}
}

func TestBuildingStateLoadsSeededDefinitionsAndOnlyCurrentLocation(t *testing.T) {
	store, db := newTestStore(t)
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-building-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	visitor, err := store.UpsertIdentity("https://accounts.google.com", "subject-building-visitor", "visitor@example.com", "Visitor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO locations (id, display_name) VALUES ('other-location', 'Other Location')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE player_locations SET location_id = 'other-location' WHERE user_id = ?`, visitor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count)
VALUES (?, 'camp', 'building_lv1', 1, 60, 12, 'under_construction', 1),
       (?, 'other-location', 'building_lv1', 1, 60, 60, 'completed', 1);`, owner.ID, visitor.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.BuildingRecipes) != 1 {
		t.Fatalf("building recipes = %+v, want one seeded recipe", state.BuildingRecipes)
	}
	recipe := state.BuildingRecipes[0]
	if recipe.ID != "building_lv1" || recipe.DisplayName != "Building Lv1" || recipe.BuildingLevel != 1 || recipe.RequiredAP != 60 || recipe.ExtensionSlotCount != 1 || len(recipe.ResourceInputs) != 1 || recipe.ResourceInputs[0].Resource.ID != "wood" || recipe.ResourceInputs[0].Quantity != 10 || len(recipe.ItemInputs) != 1 || recipe.ItemInputs[0].Item.ID != "wood_component" || recipe.ItemInputs[0].Quantity != 1 {
		t.Fatalf("seeded building recipe = %+v", recipe)
	}
	if len(state.Buildings) != 1 || state.Buildings[0].Owner.ID != owner.ID || state.Buildings[0].Recipe.ID != "building_lv1" || state.Buildings[0].BuildingLevel != 1 || state.Buildings[0].RequiredAP != 60 || state.Buildings[0].ContributedAP != 12 || state.Buildings[0].Status != "under_construction" || state.Buildings[0].ExtensionSlotCount != 1 {
		t.Fatalf("current-location buildings = %+v", state.Buildings)
	}
	if _, err := db.Exec(`INSERT INTO building_recipes (id, display_name, building_level, required_ap, extension_slot_count) VALUES ('empty', 'Empty', 1, 1, 0)`); err != nil {
		t.Fatal(err)
	}
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range state.BuildingRecipes {
		if candidate.ID == "empty" {
			t.Fatal("building recipe without inputs was exposed")
		}
	}
}

func TestInstallExtensionConsumesPackageAndPersistsSlotSnapshot(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-install", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at)
VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 60, 'completed', 1, 604800, ?);
INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity)
VALUES (?, 'sawmill_package_t1', 'active', ?, 1);`, owner.ID, now.Add(time.Hour).Unix(), owner.ID, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	state, err := store.InstallExtension(owner.ID, 1, 0, "sawmill_t1")
	if err != nil {
		t.Fatal(err)
	}
	if inventoryQuantity(state, "sawmill_package_t1") != 0 || len(state.Buildings) != 1 || len(state.Buildings[0].Extensions) != 1 {
		t.Fatalf("installed extension state = %+v", state)
	}
	extension := state.Buildings[0].Extensions[0]
	if extension.SlotIndex != 0 || extension.DefinitionID != "sawmill_t1" || extension.DisplayName != "Sawmill T1" || extension.Tier != 1 || extension.RequiredAP != 30 || extension.ContributedAP != 0 || extension.Status != "under_construction" {
		t.Fatalf("installed extension snapshot = %+v", extension)
	}
	if _, err := db.Exec(`UPDATE building_extension_definitions SET display_name = 'Edited Sawmill', tier = 2, required_ap = 99 WHERE id = 'sawmill_t1'`); err != nil {
		t.Fatal(err)
	}
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Buildings[0].Extensions[0]; got.DisplayName != "Sawmill T1" || got.Tier != 1 || got.RequiredAP != 30 {
		t.Fatalf("extension snapshot changed after definition edit = %+v", got)
	}
}

func TestInstallExtensionRejectsOwnerSlotAndPackageFailuresWithoutMutation(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-install-fail-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-install-fail-other", "other@example.com", "Other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at)
VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 60, 'completed', 2, 604800, ?);
INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity)
VALUES (?, 'sawmill_package_t1', 'active', ?, 1);`, owner.ID, now.Add(time.Hour).Unix(), owner.ID, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallExtension(other.ID, 1, 0, "sawmill_t1"); !errors.Is(err, ErrBuildingNotOwner) {
		t.Fatalf("non-owner installation error = %v, want ErrBuildingNotOwner", err)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 2, "sawmill_t1"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("out-of-range slot error = %v, want ErrInvalidArgument", err)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 0, "unknown"); !errors.Is(err, ErrExtensionDefinitionNotFound) {
		t.Fatalf("unknown definition error = %v, want ErrExtensionDefinitionNotFound", err)
	}
	after, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed extension installations changed state: before=%+v after=%+v", before, after)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 0, "sawmill_t1"); err != nil {
		t.Fatal(err)
	}
	before, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 1, "sawmill_t1"); !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("second package installation error = %v, want ErrInsufficientItem", err)
	}
	after, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) || len(after.Buildings[0].Extensions) != 1 {
		t.Fatalf("insufficient package changed state: before=%+v after=%+v", before, after)
	}
}

func TestExtensionConstructionSharesAPAndCompletesAtRequiredProgress(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-construction-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	contributor, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-construction-contributor", "contributor@example.com", "Contributor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at)
VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 60, 'completed', 1, 604800, ?);
INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity)
VALUES (?, 'sawmill_package_t1', 'active', ?, 1);`, owner.ID, now.Add(time.Hour).Unix(), owner.ID, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 0, "sawmill_t1"); err != nil {
		t.Fatal(err)
	}
	state, err := store.ContributeExtensionConstruction(contributor.ID, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-10 || state.Buildings[0].Extensions[0].ContributedAP != 10 || state.Buildings[0].Extensions[0].Status != "under_construction" {
		t.Fatalf("first extension contribution = %+v", state.Buildings[0].Extensions[0])
	}
	state, err = store.ContributeExtensionConstruction(owner.ID, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-20 || state.Buildings[0].Extensions[0].ContributedAP != 30 || state.Buildings[0].Extensions[0].Status != "completed" {
		t.Fatalf("completed extension contribution = %+v", state.Buildings[0].Extensions[0])
	}
	before, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ContributeExtensionConstruction(owner.ID, 1, 1); !errors.Is(err, ErrExtensionCompleted) {
		t.Fatalf("completed extension error = %v, want ErrExtensionCompleted", err)
	}
	after, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("completed extension contribution changed state: before=%+v after=%+v", before, after)
	}
}

func TestDisabledBuildingBlocksExtensionProgressUntilRepairAndRemovalDoesNotRefund(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-disabled", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 60, 'completed', 1, 604800, ?)`, owner.ID, now.Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'sawmill_package_t1', 'active', ?, 1)`, owner.ID, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 1)`, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 0, "sawmill_t1"); !errors.Is(err, ErrBuildingDisabled) {
		t.Fatalf("disabled installation error = %v, want ErrBuildingDisabled", err)
	}
	if _, err := db.Exec(`UPDATE buildings SET durability_expires_at = ?`, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InstallExtension(owner.ID, 1, 0, "sawmill_t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE buildings SET durability_expires_at = ?`, now.Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ContributeExtensionConstruction(owner.ID, 1, 1); !errors.Is(err, ErrBuildingDisabled) {
		t.Fatalf("disabled contribution error = %v, want ErrBuildingDisabled", err)
	}
	if _, err := store.RepairBuilding(owner.ID, 1); err != nil {
		t.Fatal(err)
	}
	state, err := store.ContributeExtensionConstruction(owner.ID, 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	if state.Buildings[0].Extensions[0].Status != "completed" {
		t.Fatalf("repaired extension state = %+v", state.Buildings[0].Extensions[0])
	}
	if _, err := store.RemoveExtension(owner.ID, 1); err != nil {
		t.Fatal(err)
	}
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings[0].Extensions) != 0 || inventoryQuantity(state, "sawmill_package_t1") != 0 {
		t.Fatalf("removed extension state = %+v", state)
	}
}

func TestExpiredBuildingCascadesItsExtensions(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-extension-cascade", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at)
VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 60, 'completed', 1, 604800, ?);
INSERT INTO building_extensions (building_id, slot_index, definition_id, display_name, tier, required_ap, contributed_ap, status)
VALUES (1, 0, 'sawmill_t1', 'Sawmill T1', 1, 30, 0, 'under_construction');`, owner.ID, now.Add(-buildingDisabledRetention-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPlayerState(owner.ID); err != nil {
		t.Fatal(err)
	}
	var buildings, extensions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM buildings`).Scan(&buildings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM building_extensions`).Scan(&extensions); err != nil {
		t.Fatal(err)
	}
	if buildings != 0 || extensions != 0 {
		t.Fatalf("expired cascade counts = buildings %d, extensions %d; want zero", buildings, extensions)
	}
}

func TestBuildSeededRecipeConsumesMixedInputsAtomically(t *testing.T) {
	store, db := newTestStore(t)
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-seeded-building", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, owner.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.Build(owner.ID, "building_lv1")
	if err != nil {
		t.Fatal(err)
	}
	if resourceQuantity(state, "wood") != 0 || inventoryQuantity(state, "wood_component") != 0 || len(state.Buildings) != 1 {
		t.Fatalf("seeded building state = %+v, want both inputs consumed", state)
	}
}

func TestBuildSeededRecipeRollsBackWhenEitherMixedInputIsInsufficient(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup string
		err   error
	}{
		{"resource", `INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, ErrInsufficientResource},
		{"item", `INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)`, ErrInsufficientItem},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, db := newTestStore(t)
			owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-seeded-building-rollback-"+test.name, "owner@example.com", "Owner")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(test.setup, owner.ID); err != nil {
				t.Fatal(err)
			}
			before, err := store.GetPlayerState(owner.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Build(owner.ID, "building_lv1"); !errors.Is(err, test.err) {
				t.Fatalf("build error = %v, want %v", err, test.err)
			}
			after, err := store.GetPlayerState(owner.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("failed build changed state: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestBuildingCompletionInitializesAndComputesDurability(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-durability-completion", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (?, 'camp', 'building_lv1', 1, 60, 59, 'under_construction', 1)`, owner.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.ContributeConstruction(owner.ID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 1 {
		t.Fatalf("buildings = %+v, want one building", state.Buildings)
	}
	building := state.Buildings[0]
	buildingDurabilitySeconds := int(expectedBuildingDurabilitySeconds)
	if building.Status != "completed" || building.MaxDurabilitySeconds != buildingDurabilitySeconds || building.DurabilityStatus != "active" || building.DurabilityRemainingSeconds != buildingDurabilitySeconds {
		t.Fatalf("completed building durability = %+v, want active seven-day durability", building)
	}
	var expiry int64
	if err := db.QueryRow(`SELECT durability_expires_at FROM buildings WHERE id = 1`).Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	if expiry != now.Unix()+int64(buildingDurabilitySeconds) {
		t.Fatalf("durability expiry = %d, want %d", expiry, now.Unix()+int64(buildingDurabilitySeconds))
	}

	now = now.Add(2 * time.Hour)
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Buildings[0].DurabilityStatus != "active" || state.Buildings[0].DurabilityRemainingSeconds != buildingDurabilitySeconds-7200 {
		t.Fatalf("elapsed durability = %+v, want two hours elapsed", state.Buildings[0])
	}
}

func TestBuildingDurabilityDisablesThenDestroysAndReleasesSlot(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-durability-lifecycle", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	expiry := now.Unix()
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 1, 60, 60, 'completed', 1, ?)`, owner.ID, expiry); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 1 || state.Buildings[0].DurabilityStatus != "disabled" || state.Buildings[0].DurabilityRemainingSeconds != 0 {
		t.Fatalf("disabled building = %+v, want retained disabled building", state.Buildings)
	}
	now = now.Add(buildingDisabledRetention - time.Second)
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 1 || state.Buildings[0].DurabilityStatus != "disabled" {
		t.Fatalf("building before disabled retention boundary = %+v, want retained disabled building", state.Buildings)
	}
	now = now.Add(time.Second)
	state, err = store.GetPlayerState(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 0 {
		t.Fatalf("destroyed buildings = %+v, want none", state.Buildings)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM buildings WHERE owner_id = ? AND location_id = 'camp'`, owner.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("destroyed row count = %d, want zero", count)
	}
}

func TestBuildingDurabilitySchemaUpgradeBackfillsCompletedRows(t *testing.T) {
	db, err := sql.Open("sqlite", "file:building-durability-schema-upgrade?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC).Unix()
	if _, err := db.Exec(`
CREATE TABLE identities (id INTEGER PRIMARY KEY, issuer TEXT NOT NULL, subject TEXT NOT NULL, email TEXT NOT NULL, display_name TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE player_ap (user_id INTEGER PRIMARY KEY, full_timestamp INTEGER NOT NULL);
CREATE TABLE locations (id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
CREATE TABLE player_locations (user_id INTEGER PRIMARY KEY, location_id TEXT NOT NULL);
CREATE TABLE building_recipes (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, building_level INTEGER NOT NULL, required_ap INTEGER NOT NULL, extension_slot_count INTEGER NOT NULL);
CREATE TABLE building_recipe_resource_inputs (recipe_id TEXT NOT NULL, resource_id TEXT NOT NULL, quantity INTEGER NOT NULL, PRIMARY KEY (recipe_id, resource_id));
CREATE TABLE building_recipe_item_inputs (recipe_id TEXT NOT NULL, item_id TEXT NOT NULL, quantity INTEGER NOT NULL, PRIMARY KEY (recipe_id, item_id));
CREATE TABLE buildings (id INTEGER PRIMARY KEY AUTOINCREMENT, owner_id INTEGER NOT NULL, location_id TEXT NOT NULL, recipe_id TEXT NOT NULL, building_level INTEGER NOT NULL, required_ap INTEGER NOT NULL, contributed_ap INTEGER NOT NULL, status TEXT NOT NULL, extension_slot_count INTEGER NOT NULL, UNIQUE (owner_id, location_id));
INSERT INTO identities VALUES (41, 'https://accounts.google.com', 'legacy-building', 'person@example.com', 'Person', ?, ?);
INSERT INTO player_ap VALUES (41, ?);
INSERT INTO locations VALUES ('camp', 'Camp'), ('legacy-location', 'Legacy Location');
INSERT INTO player_locations VALUES (41, 'camp');
INSERT INTO building_recipes VALUES ('legacy-building', 'Legacy Building', 1, 60, 1);
INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (41, 'camp', 'legacy-building', 1, 60, 60, 'completed', 1), (41, 'legacy-location', 'legacy-building', 1, 60, 0, 'under_construction', 1);`, createdAt, createdAt, createdAt); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC().Unix()
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	var maxDurability int64
	var displayName string
	var expiry sql.NullInt64
	if err := db.QueryRow(`SELECT display_name, max_durability_seconds, durability_expires_at FROM buildings WHERE id = 1`).Scan(&displayName, &maxDurability, &expiry); err != nil {
		t.Fatal(err)
	}
	if displayName != "Legacy Building" || maxDurability != expectedBuildingDurabilitySeconds || !expiry.Valid || expiry.Int64 < before+expectedBuildingDurabilitySeconds-2 || expiry.Int64 > time.Now().UTC().Unix()+expectedBuildingDurabilitySeconds+2 {
		t.Fatalf("migrated completed building = name %q max %d expiry %v", displayName, maxDurability, expiry)
	}
	var underConstructionExpiry sql.NullInt64
	if err := db.QueryRow(`SELECT durability_expires_at FROM buildings WHERE id = 2`).Scan(&underConstructionExpiry); err != nil {
		t.Fatal(err)
	}
	if underConstructionExpiry.Valid {
		t.Fatalf("under-construction expiry = %v, want NULL", underConstructionExpiry)
	}
	store.now = func() time.Time { return time.Unix(before, 0).UTC() }
	state, err := store.GetPlayerState(41)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 1 || state.Buildings[0].DurabilityStatus != "active" {
		t.Fatalf("migrated state = %+v, want active completed building at current location", state.Buildings)
	}
}

func TestRepairBuildingAllowsAnyPlayerAndUpdatesAllStateAtomically(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-repair-owner", "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	repairer, err := store.UpsertIdentity("https://accounts.google.com", "subject-repairer", "repairer@example.com", "Repairer")
	if err != nil {
		t.Fatal(err)
	}
	expiry := now.Add(2 * time.Hour).Unix()
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 1, 60, 60, 'completed', 1, ?)`, owner.ID, expiry); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2)`, repairer.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.RepairBuilding(repairer.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-buildingRepairAPCost || resourceQuantity(state, "wood") != 1 || state.RepairComputation == nil {
		t.Fatalf("repair state = %+v, want AP and Wood cost plus computation", state)
	}
	computation := state.RepairComputation
	if computation.BuildingID != 1 || computation.PriorDurabilityStatus != "active" || computation.AddedSeconds != int(buildingRepairDuration/time.Second) || computation.ResultingRemainingSeconds != int(3*time.Hour/time.Second) || computation.APCost != buildingRepairAPCost || computation.WoodCost != buildingRepairWoodCost {
		t.Fatalf("repair computation = %+v", computation)
	}
	building := state.Buildings[0]
	if building.DurabilityStatus != "active" || building.DurabilityRemainingSeconds != int(3*time.Hour/time.Second) {
		t.Fatalf("repaired building = %+v, want three hours remaining", building)
	}
	var storedExpiry int64
	if err := db.QueryRow(`SELECT durability_expires_at FROM buildings WHERE id = 1`).Scan(&storedExpiry); err != nil {
		t.Fatal(err)
	}
	if storedExpiry != now.Add(3*time.Hour).Unix() {
		t.Fatalf("stored expiry = %d, want %d", storedExpiry, now.Add(3*time.Hour).Unix())
	}
}

func TestRepairBuildingRevivesDisabledBuildingAndClampsWithoutRefund(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-repair-disabled", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 1, 60, 60, 'completed', 1, 7200, ?)`, identity.ID, now.Add(-time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.RepairBuilding(identity.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Buildings[0].DurabilityRemainingSeconds != 3600 || state.RepairComputation.PriorDurabilityStatus != "disabled" {
		t.Fatalf("revived building = %+v, want one hour remaining", state.Buildings[0])
	}

	now = now.Add(2 * time.Hour)
	if _, err := db.Exec(`UPDATE buildings SET durability_expires_at = ? WHERE id = 1`, now.Add(7000*time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	state, err = store.RepairBuilding(identity.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Buildings[0].DurabilityRemainingSeconds != 7200 || state.AP != maxAP-buildingRepairAPCost || resourceQuantity(state, "wood") != 0 {
		t.Fatalf("clamped repair state = %+v, want full cap with full costs", state)
	}
}

func TestRepairBuildingRejectsInvalidTargetsAndPreservesState(t *testing.T) {
	for _, test := range []struct {
		name       string
		setup      func(*testing.T, *Store, *sql.DB, Identity, time.Time)
		repairerID func(Identity, Identity) int64
		buildingID int64
		wantErr    error
	}{
		{
			name: "remote",
			setup: func(t *testing.T, store *Store, db *sql.DB, owner Identity, now time.Time) {
				if _, err := db.Exec(`INSERT INTO locations (id, display_name) VALUES ('remote-repair', 'Remote Repair'); INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, durability_expires_at) VALUES (?, 'remote-repair', 'building_lv1', 1, 60, 60, 'completed', 1, ?)`, owner.ID, now.Add(time.Hour).Unix()); err != nil {
					t.Fatal(err)
				}
			},
			repairerID: func(owner, _ Identity) int64 { return owner.ID }, buildingID: 1, wantErr: ErrBuildingRemote,
		},
		{
			name: "under-construction",
			setup: func(t *testing.T, store *Store, db *sql.DB, owner Identity, now time.Time) {
				if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (?, 'camp', 'building_lv1', 1, 60, 0, 'under_construction', 1)`, owner.ID); err != nil {
					t.Fatal(err)
				}
			},
			repairerID: func(owner, _ Identity) int64 { return owner.ID }, buildingID: 1, wantErr: ErrBuildingUnderConstruction,
		},
		{
			name: "insufficient-ap",
			setup: func(t *testing.T, store *Store, db *sql.DB, owner Identity, now time.Time) {
				if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 1, 60, 60, 'completed', 1, ?)`, owner.ID, now.Add(time.Hour).Unix()); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, now.Add(2991*time.Minute).Unix(), owner.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 1)`, owner.ID); err != nil {
					t.Fatal(err)
				}
			},
			repairerID: func(owner, _ Identity) int64 { return owner.ID }, buildingID: 1, wantErr: ErrInsufficientAP,
		},
		{
			name: "insufficient-wood",
			setup: func(t *testing.T, store *Store, db *sql.DB, owner Identity, now time.Time) {
				if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 1, 60, 60, 'completed', 1, ?)`, owner.ID, now.Add(time.Hour).Unix()); err != nil {
					t.Fatal(err)
				}
			},
			repairerID: func(owner, _ Identity) int64 { return owner.ID }, buildingID: 1, wantErr: ErrInsufficientResource,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, db := newTestStore(t)
			now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
			store.now = func() time.Time { return now }
			owner, err := store.UpsertIdentity("https://accounts.google.com", "subject-repair-reject-"+test.name, "owner@example.com", "Owner")
			if err != nil {
				t.Fatal(err)
			}
			other, err := store.UpsertIdentity("https://accounts.google.com", "subject-repair-other-"+test.name, "other@example.com", "Other")
			if err != nil {
				t.Fatal(err)
			}
			if test.name == "remote" {
				if _, err := db.Exec(`UPDATE player_locations SET location_id = 'forest_edge' WHERE user_id = ?`, other.ID); err != nil {
					t.Fatal(err)
				}
			}
			test.setup(t, store, db, owner, now)
			before, err := store.GetPlayerState(owner.ID)
			if err != nil {
				t.Fatal(err)
			}
			actorID := test.repairerID(owner, other)
			if _, err := store.RepairBuilding(actorID, test.buildingID); !errors.Is(err, test.wantErr) {
				t.Fatalf("repair error = %v, want %v", err, test.wantErr)
			}
			after, err := store.GetPlayerState(owner.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("failed repair changed owner state: before=%+v after=%+v", before, after)
			}
		})
	}
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

func TestCraftRejectsInsufficientAPWithoutChangingAnyState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-craft-ap", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity) VALUES ('ap_limited', 'AP Limited', 10, 'wood_component', 1);
INSERT INTO crafting_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES ('ap_limited', 'wood', 2), ('ap_limited', 'stone', 3);
INSERT INTO crafting_recipe_item_inputs (recipe_id, item_id, quantity) VALUES ('ap_limited', 'wood', 1);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2), (?, 'stone', 3);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1);`, identity.ID, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Craft(identity.ID, "ap_limited"); !errors.Is(err, ErrInsufficientAP) {
		t.Fatalf("craft with insufficient AP error = %v, want ErrInsufficientAP", err)
	}
	after, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("insufficient-AP craft changed AP, resources, items, or output: before=%+v after=%+v", before, after)
	}
}

func TestCraftRejectsInsufficientItemWithoutChangingAnyState(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-craft-item", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity) VALUES ('item_limited', 'Item Limited', 10, 'wood_component', 1);
INSERT INTO crafting_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES ('item_limited', 'wood', 2), ('item_limited', 'stone', 3);
INSERT INTO crafting_recipe_item_inputs (recipe_id, item_id, quantity) VALUES ('item_limited', 'wood', 1), ('item_limited', 'wood_component', 1);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2), (?, 'stone', 3);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1);`, identity.ID, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Craft(identity.ID, "item_limited"); !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("craft with insufficient Item input error = %v, want ErrInsufficientItem", err)
	}
	after, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("insufficient-Item craft changed AP, resources, items, or output: before=%+v after=%+v", before, after)
	}
}

func TestCraftingSchemaUpgradePreservesExistingPlayerState(t *testing.T) {
	db, err := sql.Open("sqlite", "file:crafting-schema-upgrade?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 27, 11, 0, 0, 0, time.UTC).Unix()
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
	store.now = func() time.Time { return time.Unix(createdAt, 0).UTC() }
	state, err := store.GetPlayerState(41)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "legacy-location" || state.AP != maxAP || len(state.CraftingRecipes) != 2 {
		t.Fatalf("schema upgrade changed existing player state or omitted recipes: %+v", state)
	}
	seen := map[string]bool{}
	for _, recipe := range state.CraftingRecipes {
		seen[recipe.ID] = true
	}
	if !seen["wood_component"] || !seen["sawmill_package_t1"] {
		t.Fatalf("schema upgrade omitted seeded recipes: %+v", state.CraftingRecipes)
	}
	var buildingTableCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('building_recipes', 'building_recipe_resource_inputs', 'building_recipe_item_inputs', 'buildings')").Scan(&buildingTableCount); err != nil {
		t.Fatal(err)
	}
	if buildingTableCount != 4 {
		t.Fatalf("schema upgrade building tables = %d, want 4", buildingTableCount)
	}
}

func TestStorePersistsAllAbsoluteTimesAsUnixSeconds(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 987_654_321, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "seconds", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	oauthExpiresAt := now.Add(10 * time.Minute)
	if err := store.CreateOAuthAttempt("seconds-state", "nonce", "verifier", oauthExpiresAt); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeOAuthAttempt("seconds-state"); err != nil {
		t.Fatal(err)
	}
	sessionExpiresAt := now.Add(time.Hour)
	if err := store.CreateSession(identity.ID, "seconds-session", sessionExpiresAt); err != nil {
		t.Fatal(err)
	}

	values := make([]int64, 7)
	if err := db.QueryRow("SELECT created_at, updated_at FROM identities WHERE id = ?", identity.ID).Scan(&values[0], &values[1]); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&values[2]); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT expires_at, consumed_at FROM oauth_attempts WHERE state_hash = ?", hashSecret("seconds-state")).Scan(&values[3], &values[4]); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT expires_at, created_at FROM sessions WHERE token_hash = ?", hashSecret("seconds-session")).Scan(&values[5], &values[6]); err != nil {
		t.Fatal(err)
	}
	want := []int64{now.Unix(), now.Unix(), now.Unix(), oauthExpiresAt.Unix(), now.Unix(), sessionExpiresAt.Unix(), now.Unix()}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("persisted timestamps = %v, want Unix seconds %v", values, want)
	}
}

func TestNewStoreMigratesPersistentTimestampsToUnixSecondsOnce(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 987_654_321, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "legacy-timestamps", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	createdAt := now.Add(-time.Hour)
	updatedAt := now.Add(-time.Minute)
	fullAt := now.Add(10 * time.Minute)
	oauthExpiresAt := now.Add(10 * time.Minute)
	oauthConsumedAt := now.Add(-time.Minute)
	sessionExpiresAt := now.Add(time.Hour)
	sessionCreatedAt := now.Add(-time.Hour)
	if _, err := db.Exec("UPDATE identities SET created_at = ?, updated_at = ? WHERE id = ?", createdAt.UnixNano(), updatedAt.UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", fullAt.UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO oauth_attempts (state_hash, nonce, verifier, expires_at, consumed_at) VALUES (?, '', '', ?, ?)", []byte("legacy-oauth"), oauthExpiresAt.UnixNano(), oauthConsumedAt.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)", []byte("legacy-session"), identity.ID, sessionExpiresAt.UnixNano(), sessionCreatedAt.UnixNano()); err != nil {
		t.Fatal(err)
	}

	readTimestamps := func() []int64 {
		t.Helper()
		values := make([]int64, 7)
		if err := db.QueryRow("SELECT created_at, updated_at FROM identities WHERE id = ?", identity.ID).Scan(&values[0], &values[1]); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&values[2]); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT expires_at, consumed_at FROM oauth_attempts WHERE state_hash = ?", []byte("legacy-oauth")).Scan(&values[3], &values[4]); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow("SELECT expires_at, created_at FROM sessions WHERE token_hash = ?", []byte("legacy-session")).Scan(&values[5], &values[6]); err != nil {
			t.Fatal(err)
		}
		return values
	}

	var migratedStore *Store
	logOutput := captureStdout(t, func() {
		migratedStore, err = NewStore(db)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logOutput, "user_id=anonymous action=timestamp_migration outcome=success converted_values=7 request_id=unavailable") {
		t.Fatalf("timestamp migration log = %q", logOutput)
	}
	migratedStore.now = func() time.Time { return now }
	want := []int64{createdAt.Unix(), updatedAt.Unix(), fullAt.Unix(), oauthExpiresAt.Unix(), oauthConsumedAt.Unix(), sessionExpiresAt.Unix(), sessionCreatedAt.Unix()}
	if got := readTimestamps(); !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated timestamps = %v, want Unix seconds %v", got, want)
	}
	if ap, err := migratedStore.GetAP(identity.ID); err != nil || ap != 2990 {
		t.Fatalf("AP after timestamp migration = %d, %v; want 2990", ap, err)
	}
	logOutput = captureStdout(t, func() {
		_, err = NewStore(db)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logOutput, "converted_values=0") {
		t.Fatalf("idempotent timestamp migration log = %q", logOutput)
	}
	if got := readTimestamps(); !reflect.DeepEqual(got, want) {
		t.Fatalf("second initialization changed Unix seconds: got %v want %v", got, want)
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
	if fullTimestamp != now.Unix() {
		t.Fatalf("new player full timestamp = %d, want %d", fullTimestamp, now.Unix())
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
		{name: "just over one minute", fullAt: now.Add(time.Minute + time.Second), wantAP: 2998},
		{name: "at full boundary", fullAt: now.Add(3000 * time.Minute), wantAP: 0},
		{name: "past full boundary", fullAt: now.Add(-time.Second), wantAP: 3000},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", test.fullAt.Unix(), identity.ID); err != nil {
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
	if fullTimestamp != now.Add(time.Minute).Unix() {
		t.Fatalf("rest full timestamp = %d, want %d", fullTimestamp, now.Add(time.Minute).Unix())
	}

	reloaded, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = func() time.Time { return now }
	if ap, err := reloaded.GetAP(identity.ID); err != nil || ap != 2999 {
		t.Fatalf("persisted rest AP = %d, %v; want 2999", ap, err)
	}

	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(3000*time.Minute).Unix(), identity.ID); err != nil {
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
	if fullTimestamp != now.Add(3000*time.Minute).Unix() {
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
	if fullTimestamp > time.Now().UTC().Unix() {
		t.Fatalf("backfilled full timestamp is in the future: %d", fullTimestamp)
	}
	backfillTime := unixSeconds(fullTimestamp)
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

	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", int64(1), userID); err != nil {
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
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(2999*time.Minute).Unix(), identity.ID); err != nil {
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
	before := now.Add(maxAP * time.Minute).Unix()
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
	if _, err := db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add((maxAP-10)*time.Minute).Unix(), identity.ID); err != nil {
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
					_, err = tx.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = 1", time.Date(2026, 8, 25, 12, 20, 0, 0, time.UTC).Unix())
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
	before := now.Add(maxAP * time.Minute).Unix()
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
	before := now.Unix()
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
	before := now.Add(maxAP * time.Minute).Unix()
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

func TestConvertSelectsHandMethodAtAnyLocationAndRollsEssencePerInput(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	store.essenceRoll = func() int { return 0 }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-method", "method@example.com", "Method")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 3)", identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err := store.Convert(identity.ID, "hand_wood_t1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-20-30 || resourceQuantity(state, "wood") != 3 || inventoryQuantity(state, "wood") != 0 || inventoryQuantity(state, "wood_essence_t1") != 3 {
		t.Fatalf("method conversion state = %+v", state)
	}
}

func TestConvertSawmillRequiresProviderAndAtomicallyUsesBuildingDurability(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	store.essenceRoll = func() int { return 10000 }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-sawmill", "sawmill@example.com", "Sawmill")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 1, 60, 60, 'completed', 1, 604800, ?); INSERT INTO building_extensions (building_id, slot_index, definition_id, display_name, tier, required_ap, contributed_ap, status) VALUES (1, 0, 'sawmill_t1', 'Sawmill T1', 1, 30, 30, 'completed'); INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 6)`, identity.ID, now.Add(100*time.Second).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Convert(identity.ID, "sawmill_wood_t1", 7, int64(1)); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("capacity error = %v", err)
	}
	state, err := store.Convert(identity.ID, "sawmill_wood_t1", 6, int64(1))
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-30 || resourceQuantity(state, "wood") != 6 || inventoryQuantity(state, "wood") != 0 {
		t.Fatalf("sawmill state = %+v", state)
	}
	var expiry int64
	if err := db.QueryRow("SELECT durability_expires_at FROM buildings WHERE id = 1").Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	if expiry != now.Add(40*time.Second).Unix() {
		t.Fatalf("sawmill durability expiry = %d, want %d", expiry, now.Add(40*time.Second).Unix())
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
	if !attempt.ExpiresAt.Equal(expiresAt.UTC().Truncate(time.Second)) {
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

func groundItemQuantity(state PlayerState, itemID string) int {
	for _, item := range state.GroundItems {
		if item.Item.ID == itemID {
			return item.Quantity
		}
	}
	return 0
}

func groundResourceQuantity(state PlayerState, resourceID string) int {
	for _, resource := range state.GroundResources {
		if resource.Resource.ID == resourceID {
			return resource.Quantity
		}
	}
	return 0
}

func TestGroundTransfersPersistByLocationAndKeepAPUnchanged(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-transfer", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-reader", "reader@example.com", "Reader")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 5);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 7);`, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.GroundItems) != 0 || len(before.GroundResources) != 0 {
		t.Fatalf("initial ground holdings = %+v, want empty", before)
	}
	if _, err := store.Drop(identity.ID, "item", "wood", 3, "active"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Drop(identity.ID, "resource", "wood", 4, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Drop(identity.ID, "item", "wood", 1, "active"); err != nil {
		t.Fatal(err)
	}
	afterDrop, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterDrop.AP != before.AP || groundItemQuantity(afterDrop, "wood") != 4 || groundResourceQuantity(afterDrop, "wood") != 4 || inventoryQuantity(afterDrop, "wood") != 1 || resourceQuantity(afterDrop, "wood") != 3 {
		t.Fatalf("drop state = %+v, want merged location holdings and unchanged AP", afterDrop)
	}
	otherState, err := store.GetPlayerState(other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if groundItemQuantity(otherState, "wood") != 4 || groundResourceQuantity(otherState, "wood") != 4 {
		t.Fatalf("other player ground state = %+v, want shared camp holdings", otherState)
	}
	reloaded, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	persistedState, err := reloaded.GetPlayerState(other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if groundItemQuantity(persistedState, "wood") != 4 || groundResourceQuantity(persistedState, "wood") != 4 {
		t.Fatalf("reloaded ground state = %+v, want persisted public holdings", persistedState)
	}
	for _, table := range []string{"ground_items", "ground_resources"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?)`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		wantColumns := 3
		if table == "ground_items" {
			wantColumns = 5
		}
		if count != wantColumns {
			t.Fatalf("%s columns = %d, want %d", table, count, wantColumns)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name IN ('owner_id', 'capacity', 'max_capacity', 'reservation_id')`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s has ownership or capacity columns", table)
		}
	}
	if _, err := db.Exec(`INSERT INTO locations (id, display_name) VALUES ('ground-remote', 'Ground Remote'); UPDATE player_locations SET location_id = 'ground-remote' WHERE user_id = ?`, identity.ID); err != nil {
		t.Fatal(err)
	}
	remoteState, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remoteState.GroundItems) != 0 || len(remoteState.GroundResources) != 0 {
		t.Fatalf("remote ground state = %+v, want isolated holdings", remoteState)
	}
	if _, err := db.Exec(`UPDATE player_locations SET location_id = 'camp' WHERE user_id = ?`, identity.ID); err != nil {
		t.Fatal(err)
	}
	picked, err := store.Pickup(identity.ID, "item", "wood", 4, "active")
	if err != nil {
		t.Fatal(err)
	}
	if picked.AP != before.AP || groundItemQuantity(picked, "wood") != 0 || inventoryQuantity(picked, "wood") != 5 {
		t.Fatalf("pickup item state = %+v, want complete transfer without AP change", picked)
	}
	picked, err = store.Pickup(identity.ID, "resource", "wood", 4, "")
	if err != nil {
		t.Fatal(err)
	}
	if groundResourceQuantity(picked, "wood") != 0 || resourceQuantity(picked, "wood") != 7 {
		t.Fatalf("pickup resource state = %+v, want complete Resource transfer", picked)
	}
	var groundRows int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM ground_items) + (SELECT COUNT(*) FROM ground_resources)`).Scan(&groundRows); err != nil {
		t.Fatal(err)
	}
	if groundRows != 0 {
		t.Fatalf("zero quantity ground rows remain: %d", groundRows)
	}
}

func TestGroundTransferRejectsInvalidOrInsufficientSourcesWithoutMutation(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-reject", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 2)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2); INSERT INTO ground_resources (location_id, resource_id, quantity) VALUES ('camp', 'wood', 2)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		pickup    bool
		assetType string
		assetID   string
		quantity  int
		wantErr   error
	}{
		{name: "zero quantity", assetType: "item", assetID: "wood", quantity: 0, wantErr: ErrInvalidArgument},
		{name: "unknown type", assetType: "currency", assetID: "wood", quantity: 1, wantErr: ErrInvalidArgument},
		{name: "unknown item", assetType: "item", assetID: "missing", quantity: 1, wantErr: ErrTransferAssetNotFound},
		{name: "unknown resource", assetType: "resource", assetID: "missing", quantity: 1, wantErr: ErrTransferAssetNotFound},
		{name: "insufficient item", assetType: "item", assetID: "wood", quantity: 3, wantErr: ErrInsufficientTransferAsset},
		{name: "insufficient resource", assetType: "resource", assetID: "wood", quantity: 3, wantErr: ErrInsufficientTransferAsset},
		{name: "insufficient ground item", pickup: true, assetType: "item", assetID: "wood", quantity: 1, wantErr: ErrInsufficientTransferAsset},
		{name: "insufficient ground resource", pickup: true, assetType: "resource", assetID: "wood", quantity: 3, wantErr: ErrInsufficientTransferAsset},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var state PlayerState
			var err error
			if test.pickup {
				status := ""
				if test.assetType == "item" {
					status = "active"
				}
				state, err = store.Pickup(identity.ID, test.assetType, test.assetID, test.quantity, status)
			} else {
				status := ""
				if test.assetType == "item" {
					status = "active"
				}
				state, err = store.Drop(identity.ID, test.assetType, test.assetID, test.quantity, status)
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("transfer error = %v, want %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(state, PlayerState{}) {
				t.Fatalf("failed transfer returned state = %+v, want empty state", state)
			}
			after, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("failed transfer changed state: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestItemActionsCreateFullDurabilityAndNeverConsumeExpiredInputs(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-action-durability", "person@example.com", "Person")
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
	if len(state.Inventory) != 1 || state.Inventory[0].DurabilityStatus != "active" || state.Inventory[0].DurabilityRemainingSeconds == nil || *state.Inventory[0].DurabilityRemainingSeconds != int(expectedItemDurabilitySeconds) {
		t.Fatalf("gathered item = %+v, want a full-lifetime active stack", state.Inventory)
	}
	if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err = store.Craft(identity.ID, "wood_component")
	if err != nil {
		t.Fatal(err)
	}
	var crafted InventoryItem
	for _, item := range state.Inventory {
		if item.Item.ID == "wood_component" {
			crafted = item
		}
	}
	if crafted.Quantity != 1 || crafted.DurabilityStatus != "active" || crafted.DurabilityRemainingSeconds == nil || *crafted.DurabilityRemainingSeconds != int(expectedItemDurabilitySeconds) {
		t.Fatalf("crafted item = %+v, want a full-lifetime active stack", crafted)
	}

	converter, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-converter", "converter@example.com", "Converter")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood', 'expired', ?, 1)`, converter.ID, now.Add(itemExpiredRetention-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(converter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Convert(converter.ID); !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("convert with expired input error = %v, want ErrInsufficientItem", err)
	}
	after, err := store.GetPlayerState(converter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("expired conversion changed AP or holdings: before=%+v after=%+v", before, after)
	}
}

func TestSuccessfulActionsPreserveItemDurabilityCleanupMetadata(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Store, *sql.DB, int64, time.Time) (PlayerState, error)
	}{
		{name: "pickup", run: func(store *Store, db *sql.DB, userID int64, now time.Time) (PlayerState, error) {
			_, err := db.Exec(`INSERT INTO ground_items (location_id, item_id, durability_status, status_expires_at, quantity) VALUES ('camp', 'wood', 'active', ?, 1)`, now.Add(time.Hour).Unix())
			if err != nil {
				return PlayerState{}, err
			}
			return store.Pickup(userID, "item", "wood", 1, "active")
		}},
		{name: "move", run: func(store *Store, _ *sql.DB, userID int64, _ time.Time) (PlayerState, error) {
			return store.Move(userID, "forest_edge")
		}},
		{name: "convert", run: func(store *Store, db *sql.DB, userID int64, now time.Time) (PlayerState, error) {
			_, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood', 'active', ?, 1)`, userID, now.Add(time.Hour).Unix())
			if err != nil {
				return PlayerState{}, err
			}
			return store.Convert(userID)
		}},
		{name: "craft", run: func(store *Store, db *sql.DB, userID int64, _ time.Time) (PlayerState, error) {
			_, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)`, userID)
			if err != nil {
				return PlayerState{}, err
			}
			return store.Craft(userID, "wood_component")
		}},
		{name: "build", run: func(store *Store, db *sql.DB, userID int64, now time.Time) (PlayerState, error) {
			_, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood_component', 'active', ?, 1)`, userID, now.Add(time.Hour).Unix())
			if err != nil {
				return PlayerState{}, err
			}
			if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)`, userID); err != nil {
				return PlayerState{}, err
			}
			return store.Build(userID, "building_lv1")
		}},
		{name: "construction contribution", run: func(store *Store, db *sql.DB, userID int64, _ time.Time) (PlayerState, error) {
			_, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds) VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 0, 'under_construction', 1, ?)`, userID, int(buildingDefaultDurability/time.Second))
			if err != nil {
				return PlayerState{}, err
			}
			return store.ContributeConstruction(userID, 1, 1)
		}},
		{name: "repair", run: func(store *Store, db *sql.DB, userID int64, now time.Time) (PlayerState, error) {
			_, err := db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds, durability_expires_at) VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, 60, 'completed', 1, ?, ?)`, userID, int(buildingDefaultDurability/time.Second), now.Add(time.Hour).Unix())
			if err != nil {
				return PlayerState{}, err
			}
			if _, err := db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 1)`, userID); err != nil {
				return PlayerState{}, err
			}
			return store.RepairBuilding(userID, 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, db := newTestStore(t)
			now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
			store.now = func() time.Time { return now }
			identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-cleanup-"+test.name, test.name+"@example.com", "Person")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO items (id, display_name, weight_units, max_durability_seconds) VALUES ('cleanup_item', 'Cleanup Item', 1, ?)`, expectedItemDurabilitySeconds); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'cleanup_item', 'active', ?, 1)`, identity.ID, now.Add(-time.Hour).Unix()); err != nil {
				t.Fatal(err)
			}
			state, err := test.run(store, db, identity.ID, now)
			if err != nil {
				t.Fatalf("action error = %v", err)
			}
			if len(state.ItemDurabilityCleanups) != 1 {
				t.Fatalf("cleanup metadata = %+v, want one expiration", state.ItemDurabilityCleanups)
			}
			cleanup := state.ItemDurabilityCleanups[0]
			if cleanup.Holding != "inventory" || cleanup.ItemID != "cleanup_item" || cleanup.Quantity != 1 || cleanup.Action != "expired" || cleanup.ExpiredAt != now.Add(-time.Hour).Unix() {
				t.Fatalf("cleanup metadata = %+v", cleanup)
			}
		})
	}
}

func TestItemTransfersPreservePartialDurabilityAndRejectExpiredPickup(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-transfer-durability", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	activeInventoryExpiry := now.Add(101 * time.Second).Unix()
	activeGroundExpiry := now.Add(100 * time.Second).Unix()
	expiredInventoryExpiry := now.Add(200 * time.Second).Unix()
	expiredGroundExpiry := now.Add(300 * time.Second).Unix()
	if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood', 'active', ?, 3), (?, 'wood', 'expired', ?, 2)`, identity.ID, activeInventoryExpiry, identity.ID, expiredInventoryExpiry); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ground_items (location_id, item_id, durability_status, status_expires_at, quantity) VALUES ('camp', 'wood', 'active', ?, 2), ('camp', 'wood', 'expired', ?, 4)`, activeGroundExpiry, expiredGroundExpiry); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Drop(identity.ID, "item", "wood", 1, "active"); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != before.AP || inventoryStackQuantity(state, "wood", "active") != 2 || groundStackQuantity(state, "wood", "active") != 3 {
		t.Fatalf("partial active drop state = %+v, want source and destination quantities preserved", state)
	}
	if remaining := stackRemaining(state.Inventory, "wood", "active"); remaining != 101 {
		t.Fatalf("partial active source remaining = %d, want 101 seconds", remaining)
	}
	if remaining := groundStackRemaining(state.GroundItems, "wood", "active"); remaining != 100 {
		t.Fatalf("weighted active destination remaining = %d, want floored 100 seconds", remaining)
	}
	if _, err := store.Drop(identity.ID, "item", "wood", 1, "expired"); err != nil {
		t.Fatal(err)
	}
	state, err = store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inventoryStackQuantity(state, "wood", "expired") != 1 || groundStackQuantity(state, "wood", "expired") != 5 || stackRetention(state.GroundItems, "wood") != 300 {
		t.Fatalf("expired drop state = %+v, want merged quantity and latest deletion deadline", state)
	}
	before, err = store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pickup(identity.ID, "item", "wood", 1, "expired"); !errors.Is(err, ErrInsufficientTransferAsset) {
		t.Fatalf("expired pickup error = %v, want ErrInsufficientTransferAsset", err)
	}
	after, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("expired pickup changed AP or holdings: before=%+v after=%+v", before, after)
	}
	if _, err := store.Drop(identity.ID, "resource", "wood", 1, "active"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("resource transfer with item status error = %v, want ErrInvalidArgument", err)
	}
}

func inventoryStackQuantity(state PlayerState, itemID, status string) int {
	for _, item := range state.Inventory {
		if item.Item.ID == itemID && item.DurabilityStatus == status {
			return item.Quantity
		}
	}
	return 0
}

func groundStackQuantity(state PlayerState, itemID, status string) int {
	for _, item := range state.GroundItems {
		if item.Item.ID == itemID && item.DurabilityStatus == status {
			return item.Quantity
		}
	}
	return 0
}

func stackRemaining(items []InventoryItem, itemID, status string) int {
	for _, item := range items {
		if item.Item.ID == itemID && item.DurabilityStatus == status && item.DurabilityRemainingSeconds != nil {
			return *item.DurabilityRemainingSeconds
		}
	}
	return 0
}

func groundStackRemaining(items []GroundItem, itemID, status string) int {
	for _, item := range items {
		if item.Item.ID == itemID && item.DurabilityStatus == status && item.DurabilityRemainingSeconds != nil {
			return *item.DurabilityRemainingSeconds
		}
	}
	return 0
}

func stackRetention(items []GroundItem, itemID string) int {
	for _, item := range items {
		if item.Item.ID == itemID && item.DurabilityStatus == "expired" && item.RetentionRemainingSeconds != nil {
			return *item.RetentionRemainingSeconds
		}
	}
	return 0
}

func TestConcurrentGroundPickupCannotOverdraw(t *testing.T) {
	store, db := newTestStore(t)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-concurrent", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ground_items (location_id, item_id, quantity) VALUES ('camp', 'wood', 1)`); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := store.Pickup(identity.ID, "item", "wood", 1, "active")
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	insufficient := 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrInsufficientTransferAsset) {
			insufficient++
		} else {
			t.Fatalf("concurrent pickup error = %v", err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent pickup outcomes = %d successes, %d insufficient; want one each", successes, insufficient)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if groundItemQuantity(state, "wood") != 0 || inventoryQuantity(state, "wood") != 1 || state.AP != maxAP {
		t.Fatalf("concurrent pickup state = %+v, want one item and unchanged AP", state)
	}
}

func TestWeightSchemaUpgradePreservesQuantitiesAndIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", "file:weight-schema-upgrade?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Unix()
	if _, err := db.Exec(`
CREATE TABLE identities (id INTEGER PRIMARY KEY, issuer TEXT NOT NULL, subject TEXT NOT NULL, email TEXT NOT NULL, display_name TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE oauth_attempts (state_hash BLOB PRIMARY KEY, browser_token_hash BLOB, nonce TEXT NOT NULL, verifier TEXT NOT NULL, expires_at INTEGER NOT NULL, consumed_at INTEGER);
CREATE TABLE sessions (token_hash BLOB PRIMARY KEY, user_id INTEGER NOT NULL, expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE player_ap (user_id INTEGER PRIMARY KEY, full_timestamp INTEGER NOT NULL);
CREATE TABLE locations (id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
CREATE TABLE player_locations (user_id INTEGER PRIMARY KEY, location_id TEXT NOT NULL);
CREATE TABLE items (id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
CREATE TABLE resource_types (id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
CREATE TABLE player_inventory (user_id INTEGER NOT NULL, item_id TEXT NOT NULL, quantity INTEGER NOT NULL, PRIMARY KEY (user_id, item_id));
CREATE TABLE player_resources (user_id INTEGER NOT NULL, resource_id TEXT NOT NULL, quantity INTEGER NOT NULL, PRIMARY KEY (user_id, resource_id));
CREATE TABLE conversion_rules (location_id TEXT PRIMARY KEY, input_item_id TEXT NOT NULL, input_quantity INTEGER NOT NULL, resource_yield INTEGER NOT NULL, ap_cost INTEGER NOT NULL);
INSERT INTO identities VALUES (41, 'https://accounts.google.com', 'legacy-weight', 'person@example.com', 'Person', ?, ?);
INSERT INTO player_ap VALUES (41, ?);
INSERT INTO locations VALUES ('camp', 'Camp');
INSERT INTO player_locations VALUES (41, 'camp');
INSERT INTO items VALUES ('wood', 'Wood'), ('wood_component', 'Wood Component');
INSERT INTO resource_types VALUES ('wood', 'Wood'), ('stone', 'Stone');
INSERT INTO player_inventory VALUES (41, 'wood', 3);
INSERT INTO player_resources VALUES (41, 'stone', 7);`, now, now, now); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Unix(now, 0).UTC() }
	for _, table := range []string{"items", "resource_types"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = 'weight_units'", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s weight column count = %d, want one", table, count)
		}
	}
	var woodWeight, componentWeight, stoneWeight int
	if err := db.QueryRow("SELECT weight_units FROM items WHERE id = 'wood'").Scan(&woodWeight); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT weight_units FROM items WHERE id = 'wood_component'").Scan(&componentWeight); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT weight_units FROM resource_types WHERE id = 'stone'").Scan(&stoneWeight); err != nil {
		t.Fatal(err)
	}
	if woodWeight != 100 || componentWeight != 10 || stoneWeight != 1 {
		t.Fatalf("migrated weights = wood %d, component %d, stone %d; want 100, 10, 1", woodWeight, componentWeight, stoneWeight)
	}
	state, err := store.GetPlayerState(41)
	if err != nil {
		t.Fatal(err)
	}
	if state.CarriedWeight != 307 || state.MovementWeightThreshold != movementWeightThreshold {
		t.Fatalf("migrated carrying state = %+v, want weight 307 and threshold %d", state, movementWeightThreshold)
	}
	var itemQuantity, resourceQuantity int
	if err := db.QueryRow("SELECT quantity FROM player_inventory WHERE user_id = 41 AND item_id = 'wood'").Scan(&itemQuantity); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT quantity FROM player_resources WHERE user_id = 41 AND resource_id = 'stone'").Scan(&resourceQuantity); err != nil {
		t.Fatal(err)
	}
	if itemQuantity != 3 || resourceQuantity != 7 {
		t.Fatalf("migrated quantities = item %d, resource %d; want 3 and 7", itemQuantity, resourceQuantity)
	}
	if _, err := NewStore(db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT quantity FROM player_inventory WHERE user_id = 41 AND item_id = 'wood'").Scan(&itemQuantity); err != nil {
		t.Fatal(err)
	}
	if itemQuantity != 3 {
		t.Fatalf("idempotent migration changed item quantity to %d", itemQuantity)
	}
}

func TestItemDurabilitySchemaUpgradePreservesLegacyHoldingsWithFullLifetime(t *testing.T) {
	db, err := sql.Open("sqlite", "file:item-durability-schema-upgrade?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC).Unix()
	if _, err := db.Exec(`
CREATE TABLE identities (id INTEGER PRIMARY KEY, issuer TEXT NOT NULL, subject TEXT NOT NULL, email TEXT NOT NULL, display_name TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE player_ap (user_id INTEGER PRIMARY KEY, full_timestamp INTEGER NOT NULL);
CREATE TABLE locations (id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
CREATE TABLE player_locations (user_id INTEGER PRIMARY KEY, location_id TEXT NOT NULL);
CREATE TABLE items (id TEXT PRIMARY KEY, display_name TEXT NOT NULL, weight_units INTEGER NOT NULL);
CREATE TABLE player_inventory (user_id INTEGER NOT NULL, item_id TEXT NOT NULL, quantity INTEGER NOT NULL, PRIMARY KEY (user_id, item_id));
CREATE TABLE ground_items (location_id TEXT NOT NULL, item_id TEXT NOT NULL, quantity INTEGER NOT NULL, PRIMARY KEY (location_id, item_id));
INSERT INTO identities VALUES (41, 'https://accounts.google.com', 'legacy-item-durability', 'person@example.com', 'Person', ?, ?);
INSERT INTO player_ap VALUES (41, ?);
INSERT INTO locations VALUES ('camp', 'Camp');
INSERT INTO player_locations VALUES (41, 'camp');
INSERT INTO items VALUES ('wood', 'Wood', 100), ('wood_component', 'Wood Component', 10);
INSERT INTO player_inventory VALUES (41, 'wood', 3);
INSERT INTO ground_items VALUES ('camp', 'wood_component', 2);`, createdAt, createdAt, createdAt); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC().Unix()
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	var expiry int64
	var quantity int
	if err := db.QueryRow(`SELECT durability_status, status_expires_at, quantity FROM player_inventory WHERE user_id = 41 AND item_id = 'wood'`).Scan(&status, &expiry, &quantity); err != nil {
		t.Fatal(err)
	}
	if status != "active" || quantity != 3 || expiry < before+expectedItemDurabilitySeconds-2 || expiry > time.Now().UTC().Unix()+expectedItemDurabilitySeconds+2 {
		t.Fatalf("migrated player holding = status %q expiry %d quantity %d, want full lifetime from migration", status, expiry, quantity)
	}
	playerExpiry := expiry
	if err := db.QueryRow(`SELECT durability_status, status_expires_at, quantity FROM ground_items WHERE location_id = 'camp' AND item_id = 'wood_component'`).Scan(&status, &expiry, &quantity); err != nil {
		t.Fatal(err)
	}
	if status != "active" || quantity != 2 || expiry < before+expectedItemDurabilitySeconds-2 || expiry > time.Now().UTC().Unix()+expectedItemDurabilitySeconds+2 {
		t.Fatalf("migrated ground holding = status %q expiry %d quantity %d, want full lifetime from migration", status, expiry, quantity)
	}
	state, err := store.GetPlayerState(41)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Inventory) != 1 || state.Inventory[0].DurabilityStatus != "active" || state.Inventory[0].DurabilityRemainingSeconds == nil || *state.Inventory[0].DurabilityRemainingSeconds <= 0 {
		t.Fatalf("migrated inventory state = %+v, want visible active holding with remaining durability", state.Inventory)
	}
	if playerExpiry <= before {
		t.Fatalf("migrated player expiry = %d, want future deadline", playerExpiry)
	}
}

func TestItemDurabilityRepeatedInitializationPreservesActiveAndExpiredDeadlines(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-durability-repeated-init", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	activeExpiry := now.Add(17 * time.Minute).Unix()
	expiredExpiry := now.Add(11 * time.Hour).Unix()
	if _, err := db.Exec(`
INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity)
VALUES (?, 'wood', 'active', ?, 1), (?, 'wood', 'expired', ?, 2)`, identity.ID, activeExpiry, identity.ID, expiredExpiry); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(db); err != nil {
		t.Fatal(err)
	}
	var storedActiveExpiry, storedExpiredExpiry int64
	if err := db.QueryRow(`SELECT status_expires_at FROM player_inventory WHERE user_id = ? AND item_id = 'wood' AND durability_status = 'active'`, identity.ID).Scan(&storedActiveExpiry); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status_expires_at FROM player_inventory WHERE user_id = ? AND item_id = 'wood' AND durability_status = 'expired'`, identity.ID).Scan(&storedExpiredExpiry); err != nil {
		t.Fatal(err)
	}
	if storedActiveExpiry != activeExpiry || storedExpiredExpiry != expiredExpiry {
		t.Fatalf("repeated initialization changed deadlines: active %d/%d expired %d/%d", storedActiveExpiry, activeExpiry, storedExpiredExpiry, expiredExpiry)
	}
}

func TestItemDurabilityNormalizationRetainsExpiredWeightAndCleansFromActualExpiry(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-durability-lifecycle", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	activeExpiredAt := now.Add(-2 * time.Hour).Unix()
	retainedUntil := now.Add(2 * time.Hour).Unix()
	if _, err := db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood', 'active', ?, 3), (?, 'wood', 'expired', ?, 2)`, identity.ID, activeExpiredAt, identity.ID, retainedUntil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ground_items (location_id, item_id, durability_status, status_expires_at, quantity) VALUES ('camp', 'wood', 'active', ?, 4), ('camp', 'wood', 'expired', ?, 1)`, activeExpiredAt, retainedUntil); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Inventory) != 1 || state.Inventory[0].DurabilityStatus != "expired" || state.Inventory[0].Quantity != 5 || state.Inventory[0].RetentionRemainingSeconds == nil {
		t.Fatalf("normalized inventory = %+v, want one merged retained expired stack", state.Inventory)
	}
	if len(state.GroundItems) != 1 || state.GroundItems[0].DurabilityStatus != "expired" || state.GroundItems[0].Quantity != 5 {
		t.Fatalf("normalized ground items = %+v, want one merged retained expired stack", state.GroundItems)
	}
	if state.CarriedWeight != 500 {
		t.Fatalf("retained expired carrying weight = %d, want 500", state.CarriedWeight)
	}
	var storedExpiry int64
	if err := db.QueryRow(`SELECT status_expires_at FROM player_inventory WHERE user_id = ? AND item_id = 'wood' AND durability_status = 'expired'`, identity.ID).Scan(&storedExpiry); err != nil {
		t.Fatal(err)
	}
	expectedExpiry := activeExpiredAt + expectedItemExpiredRetentionSeconds
	if storedExpiry != expectedExpiry {
		t.Fatalf("merged expiry = %d, want latest source deadline %d", storedExpiry, expectedExpiry)
	}
	now = time.Unix(expectedExpiry, 0).UTC()
	state, err = store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Inventory) != 0 || len(state.GroundItems) != 0 || state.CarriedWeight != 0 {
		t.Fatalf("expired cleanup state = %+v, want no retained item or weight after deadline", state)
	}
}

func TestMoveRejectsOverweightAtomicallyAndDropCanRestoreMovement(t *testing.T) {
	store, db := newTestStore(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-overweight-move", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 11)", identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.CarriedWeight != 1100 {
		t.Fatalf("overweight state = %d, want 1100", before.CarriedWeight)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); !errors.Is(err, ErrOverweight) {
		t.Fatalf("overweight move error = %v, want ErrOverweight", err)
	}
	after, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Location.ID != "camp" || after.AP != maxAP || after.CarriedWeight != 1100 {
		t.Fatalf("rejected overweight move state = %+v, want unchanged camp, AP, and weight", after)
	}
	if _, err := store.Drop(identity.ID, "item", "wood", 1, "active"); err != nil {
		t.Fatal(err)
	}
	ready, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.CarriedWeight != movementWeightThreshold {
		t.Fatalf("drop-restored weight = %d, want threshold %d", ready.CarriedWeight, movementWeightThreshold)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatalf("move at weight threshold failed: %v", err)
	}
}
