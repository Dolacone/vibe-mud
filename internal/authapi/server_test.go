package authapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestDecodeConvertRequestStrictContract(t *testing.T) {
	legacy, reason := decodeConvertRequest(strings.NewReader(`{}`))
	if reason != "" || !legacy.Legacy || legacy.MethodID != "" || legacy.Quantity != 0 {
		t.Fatalf("legacy convert request = %#v, %q", legacy, reason)
	}
	request, reason := decodeConvertRequest(strings.NewReader(`{"method_id":"hand_wood_t1","quantity":2}`))
	if reason != "" || request.MethodID != "hand_wood_t1" || request.Quantity != 2 {
		t.Fatalf("valid convert request = %#v, %q", request, reason)
	}
	for _, body := range []string{`{"method_id":"hand_wood_t1"}`, `{"quantity":2}`, `{"provider_extension_id":7}`} {
		if _, reason := decodeConvertRequest(strings.NewReader(body)); reason != convertReasonInvalidQuantity {
			t.Fatalf("partial convert request %s reason = %q", body, reason)
		}
	}
	if _, reason := decodeConvertRequest(strings.NewReader(`{"method_id":"hand_wood_t1","quantity":2,"secret":"x"}`)); reason != convertReasonUnknownField {
		t.Fatalf("unknown field reason = %q", reason)
	}
	if _, reason := decodeConvertRequest(strings.NewReader(`{"method_id":"hand_wood_t1","quantity":0}`)); reason != convertReasonInvalidQuantity {
		t.Fatalf("invalid quantity reason = %q", reason)
	}
}

type fakeProvider struct {
	authorizationURL string
	authorization    struct {
		state, nonce, challenge string
	}
	exchangeCalls int
	lastCode      string
	lastVerifier  string
	identity      ProviderIdentity
	exchangeErr   error
}

func (p *fakeProvider) AuthorizationURL(state, nonce, codeChallenge string) (string, error) {
	p.authorization.state = state
	p.authorization.nonce = nonce
	p.authorization.challenge = codeChallenge
	if p.authorizationURL == "" {
		p.authorizationURL = "https://accounts.google.com/o/oauth2/auth"
	}
	return p.authorizationURL + "?state=" + url.QueryEscape(state), nil
}

func (p *fakeProvider) Exchange(_ context.Context, code, verifier string) (ProviderIdentity, error) {
	p.exchangeCalls++
	p.lastCode = code
	p.lastVerifier = verifier
	return p.identity, p.exchangeErr
}

func newTestServer(t *testing.T, provider *fakeProvider, now *time.Time) (*Server, *Store) {
	return newTestServerWithFrontend(t, provider, now, "https://game.example.test")
}

func newTestServerWithFrontend(t *testing.T, provider *fakeProvider, now *time.Time, frontendURL string) (*Server, *Store) {
	t.Helper()
	store, _ := newTestStore(t)
	server, err := NewServer(store, provider, Config{
		FrontendURL:     frontendURL,
		CookieSecure:    true,
		SessionTTL:      time.Hour,
		OAuthAttemptTTL: 10 * time.Minute,
		Now:             func() time.Time { return *now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return server, store
}

func playerResourceQuantity(state PlayerState, resourceID string) int {
	for _, resource := range state.Resources {
		if resource.Resource.ID == resourceID {
			return resource.Quantity
		}
	}
	return 0
}

func responseResourceQuantities(t *testing.T, body map[string]any) map[string]float64 {
	t.Helper()
	entries, ok := body["resources"].([]any)
	if !ok {
		t.Fatalf("resources response = %#v", body["resources"])
	}
	quantities := make(map[string]float64, len(entries))
	for _, entry := range entries {
		resource, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("resource response entry = %#v", entry)
		}
		definition, ok := resource["resource"].(map[string]any)
		if !ok {
			t.Fatalf("resource response definition = %#v", resource["resource"])
		}
		id, ok := definition["id"].(string)
		if !ok || id == "" {
			t.Fatalf("resource response ID = %#v", definition["id"])
		}
		quantity, ok := resource["quantity"].(float64)
		if !ok {
			t.Fatalf("resource response quantity = %#v", resource["quantity"])
		}
		quantities[id] = quantity
	}
	return quantities
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestPlayerStateResponseFiltersOptionsFromCurrentAuthoritativeState(t *testing.T) {
	const userID int64 = 42
	state := PlayerState{
		Location:                     Location{ID: "camp", DisplayName: "Camp"},
		AP:                           20,
		CarriedWeight:                movementWeightThreshold,
		MovementWeightThreshold:      movementWeightThreshold,
		Routes:                       []Route{{OriginID: "camp", DestinationID: "forest_edge", APCost: 20}, {OriginID: "camp", DestinationID: "mine", APCost: 21}},
		Inventory:                    []InventoryItem{{Item: Item{ID: "wood"}, Quantity: 10, DurabilityStatus: activeItemStatus}, {Item: Item{ID: "wood_component"}, Quantity: 1, DurabilityStatus: activeItemStatus}, {Item: Item{ID: "sawmill_package_t1"}, Quantity: 1, DurabilityStatus: activeItemStatus}},
		Resources:                    []PlayerResource{{Resource: ResourceType{ID: woodResourceIdentifier}, Quantity: 10}},
		GatheringOption:              &GatheringOption{Item: Item{ID: "wood"}, Quantity: 1, APCost: 20},
		ConversionOption:             &ConversionOption{Item: Item{ID: "wood"}, Resource: ResourceType{ID: "wood"}, InputQuantity: 1, ResourceYield: 1, APCost: 20},
		ConversionMethods:            []ConversionMethod{{ID: "hand", APCost: 20, Input: Item{ID: "wood"}, MaxInputQuantity: 1, IsGlobal: true}, {ID: "sawmill", APCost: 20, Input: Item{ID: "wood"}, MaxInputQuantity: 1, ProviderDefinitionIDs: []string{"sawmill_t1"}}, {ID: "too-expensive", APCost: 21, Input: Item{ID: "wood"}, MaxInputQuantity: 1, IsGlobal: true}},
		CraftingRecipes:              []CraftingRecipe{{ID: "craftable", BaseAPCost: 20, ResourceInputs: []CraftingResourceInput{{Resource: ResourceType{ID: "wood"}, Quantity: 10}}, ItemInputs: []CraftingItemInput{{Item: Item{ID: "wood_component"}, Quantity: 1}}}, {ID: "missing-resource", BaseAPCost: 20, ResourceInputs: []CraftingResourceInput{{Resource: ResourceType{ID: "stone"}, Quantity: 1}}}},
		BuildingRecipes:              []BuildingRecipe{{ID: "buildable", ResourceInputs: []CraftingResourceInput{{Resource: ResourceType{ID: "wood"}, Quantity: 10}}, ItemInputs: []CraftingItemInput{{Item: Item{ID: "wood_component"}, Quantity: 1}}}},
		BuildingExtensionDefinitions: []BuildingExtensionDefinition{{ID: "sawmill_t1", PackageItem: Item{ID: "sawmill_package_t1"}}, {ID: "missing-package", PackageItem: Item{ID: "missing_package"}}},
		Buildings: []Building{
			{ID: 1, Owner: BuildingOwner{ID: userID}, Status: "under_construction"},
			{ID: 2, Owner: BuildingOwner{ID: userID}, Status: "completed", DurabilityStatus: activeBuildingDurabilityStatus, ExtensionSlotCount: 2, Extensions: []BuildingExtension{{ID: 101, SlotIndex: 0, DefinitionID: "sawmill_t1", Status: "completed"}}},
			{ID: 3, Owner: BuildingOwner{ID: userID}, Status: "completed", DurabilityStatus: activeBuildingDurabilityStatus, ExtensionSlotCount: 1, Extensions: []BuildingExtension{{ID: 102, SlotIndex: 0, DefinitionID: "sawmill_t1", Status: "under_construction"}}},
		},
	}
	original := state
	filtered, availability := filterAvailableGameplayOptions(state, userID)

	if !reflect.DeepEqual(state, original) {
		t.Fatalf("availability calculation mutated authoritative state: before=%+v after=%+v", original, state)
	}
	if !reflect.DeepEqual(filtered.Routes, []Route{{OriginID: "camp", DestinationID: "forest_edge", APCost: 20}}) {
		t.Fatalf("routes = %+v, want only the affordable route", filtered.Routes)
	}
	if len(filtered.CraftingRecipes) != 1 || filtered.CraftingRecipes[0].ID != "craftable" {
		t.Fatalf("crafting recipes = %+v, want only the recipe with all inputs", filtered.CraftingRecipes)
	}
	if len(filtered.BuildingRecipes) != 0 {
		t.Fatalf("building recipes = %+v, want none when the location limit is occupied", filtered.BuildingRecipes)
	}
	if filtered.ConversionOption == nil || len(filtered.ConversionMethods) != 2 {
		t.Fatalf("conversion options = %+v/%+v, want available legacy option and methods", filtered.ConversionOption, filtered.ConversionMethods)
	}
	if got := availability.ConversionProviders["sawmill"]; !reflect.DeepEqual(got, []int64{101}) {
		t.Fatalf("sawmill providers = %v, want completed local extension 101", got)
	}
	if got := availability.InstallationTargets["sawmill_t1"]; !reflect.DeepEqual(got, []extensionInstallationTargetResponse{{BuildingID: 2, SlotIndex: 1}}) {
		t.Fatalf("installation targets = %+v, want the one empty slot", got)
	}
	if len(filtered.BuildingExtensionDefinitions) != 1 || filtered.BuildingExtensionDefinitions[0].ID != "sawmill_t1" {
		t.Fatalf("extension definitions = %+v, want only the package-backed definition", filtered.BuildingExtensionDefinitions)
	}
	wantActions := []string{"rest", "move", "gather", "convert", "craft", "contribute-construction", "repair-building", "install-extension", "contribute-extension-construction", "remove-extension"}
	gotActions := append([]string(nil), availability.Actions...)
	sort.Strings(gotActions)
	sort.Strings(wantActions)
	if !reflect.DeepEqual(gotActions, wantActions) {
		t.Fatalf("available actions = %v, want %v", availability.Actions, wantActions)
	}
	if !reflect.DeepEqual(availability.BuildingActions[1], []string{"contribute-construction"}) || !reflect.DeepEqual(availability.BuildingActions[2], []string{"repair-building", "install-extension"}) {
		t.Fatalf("building action metadata = %+v", availability.BuildingActions)
	}
	if !reflect.DeepEqual(availability.ExtensionActions[102], []string{"contribute-extension-construction", "remove-extension"}) {
		t.Fatalf("extension action metadata = %+v", availability.ExtensionActions[102])
	}

	noBuilding := state
	noBuilding.Buildings = nil
	filtered, availability = filterAvailableGameplayOptions(noBuilding, userID)
	if len(filtered.BuildingRecipes) != 1 || filtered.BuildingRecipes[0].ID != "buildable" || !containsString(availability.Actions, buildAction) {
		t.Fatalf("build availability = recipes %+v/actions %v, want buildable recipe and action", filtered.BuildingRecipes, availability.Actions)
	}
}

func TestFrontendPathStaysInRedirectButNotCORSOrigin(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		frontendURL string
		origin      string
	}{
		{"https://GAME.example.test/", "https://game.example.test"},
		{"https://game.example.test/play/", "https://game.example.test"},
	}
	for _, test := range tests {
		t.Run(test.frontendURL, func(t *testing.T) {
			provider := &fakeProvider{}
			server, _ := newTestServerWithFrontend(t, provider, &now, test.frontendURL)
			handler := server.Routes()

			corsRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			corsRequest.Header.Set("Origin", test.origin)
			corsResponse := httptest.NewRecorder()
			handler.ServeHTTP(corsResponse, corsRequest)
			if corsResponse.Header().Get("Access-Control-Allow-Origin") != test.origin {
				t.Fatalf("CORS origin = %q, want %q", corsResponse.Header().Get("Access-Control-Allow-Origin"), test.origin)
			}

			state, flowCookie := loginState(t, handler)
			provider.identity = ProviderIdentity{Issuer: "https://accounts.google.com", Subject: "subject-1", Nonce: provider.authorization.nonce}
			callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=code", nil)
			callback.AddCookie(flowCookie)
			callbackResponse := httptest.NewRecorder()
			handler.ServeHTTP(callbackResponse, callback)
			location, err := callbackResponse.Result().Location()
			if err != nil {
				t.Fatal(err)
			}
			if location.String() != test.frontendURL {
				t.Fatalf("redirect location = %q, want %q", location.String(), test.frontendURL)
			}
		})
	}
}

func loginState(t *testing.T, handler http.Handler) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusFound {
		t.Fatalf("login status = %d, want %d: %s", res.Code, http.StatusFound, res.Body.String())
	}
	location, err := res.Result().Location()
	if err != nil {
		t.Fatal(err)
	}
	return location.Query().Get("state"), res.Result().Cookies()[0]
}

func TestLoginRedirectIncludesStateNonceAndPKCE(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{}
	server, store := newTestServer(t, provider, &now)
	state, flowCookie := loginState(t, server.Routes())
	if state == "" || provider.authorization.nonce == "" || provider.authorization.challenge == "" {
		t.Fatal("login redirect did not receive state, nonce, and PKCE challenge")
	}
	attempt, err := store.ConsumeOAuthAttempt(state, flowCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.Nonce != provider.authorization.nonce {
		t.Fatal("stored nonce does not match the provider request")
	}
	if pkceChallenge(attempt.Verifier) != provider.authorization.challenge {
		t.Fatal("provider received a PKCE challenge for a different verifier")
	}
}

func TestCallbackRejectsReplayAndExpiryBeforeProviderExchange(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{identity: ProviderIdentity{Issuer: "https://accounts.google.com", Subject: "subject-1", Nonce: "wrong"}}
	server, _ := newTestServer(t, provider, &now)
	handler := server.Routes()
	state, flowCookie := loginState(t, handler)
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=code-secret", nil)
	callback.AddCookie(flowCookie)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, callback)
	if first.Code != http.StatusBadRequest || provider.exchangeCalls != 1 {
		t.Fatalf("first callback status=%d exchange calls=%d", first.Code, provider.exchangeCalls)
	}
	replay := httptest.NewRecorder()
	handler.ServeHTTP(replay, callback)
	if replay.Code != http.StatusBadRequest || provider.exchangeCalls != 1 {
		t.Fatalf("replay status=%d exchange calls=%d; replay reached provider", replay.Code, provider.exchangeCalls)
	}

	expiredState, expiredCookie := loginState(t, handler)
	now = now.Add(11 * time.Minute)
	expired := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(expiredState)+"&code=another-code", nil)
	expired.AddCookie(expiredCookie)
	expiredResponse := httptest.NewRecorder()
	handler.ServeHTTP(expiredResponse, expired)
	if expiredResponse.Code != http.StatusBadRequest || provider.exchangeCalls != 1 {
		t.Fatalf("expired callback status=%d exchange calls=%d; expired state reached provider", expiredResponse.Code, provider.exchangeCalls)
	}
}

func TestCallbackRequiresTheBrowserThatStartedLogin(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{}
	server, _ := newTestServer(t, provider, &now)
	handler := server.Routes()
	state, flowCookie := loginState(t, handler)
	provider.identity = ProviderIdentity{Issuer: "https://accounts.google.com", Subject: "subject-1", Nonce: provider.authorization.nonce}

	foreign := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=code", nil)
	foreign.AddCookie(&http.Cookie{Name: oauthFlowCookieName, Value: "foreign-browser-token"})
	foreignResponse := httptest.NewRecorder()
	handler.ServeHTTP(foreignResponse, foreign)
	if foreignResponse.Code != http.StatusBadRequest || provider.exchangeCalls != 0 {
		t.Fatalf("foreign callback status/exchange calls = %d/%d", foreignResponse.Code, provider.exchangeCalls)
	}

	original := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=code", nil)
	original.AddCookie(flowCookie)
	originalResponse := httptest.NewRecorder()
	handler.ServeHTTP(originalResponse, original)
	if originalResponse.Code != http.StatusFound || provider.exchangeCalls != 1 {
		t.Fatalf("original callback status/exchange calls = %d/%d", originalResponse.Code, provider.exchangeCalls)
	}
}

func TestCallbackRejectsMissingBrowserFlowCookie(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{}
	server, _ := newTestServer(t, provider, &now)
	handler := server.Routes()
	state, _ := loginState(t, handler)
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=code", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, callback)
	if response.Code != http.StatusBadRequest || provider.exchangeCalls != 0 {
		t.Fatalf("missing flow cookie status/exchange calls = %d/%d", response.Code, provider.exchangeCalls)
	}
}

func TestCallbackSetsConstrainedSessionCookieAndRedirectsFrontend(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{identity: ProviderIdentity{
		Issuer: "https://accounts.google.com", Subject: "subject-1", Email: "person@example.com", DisplayName: "Person", Nonce: "placeholder",
	}}
	server, store := newTestServer(t, provider, &now)
	handler := server.Routes()
	state, flowCookie := loginState(t, handler)
	provider.identity.Nonce = provider.authorization.nonce
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=authorization-code-secret", nil)
	callback.AddCookie(flowCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, callback)
	location, err := response.Result().Location()
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusFound || location.String() != "https://game.example.test" {
		t.Fatalf("callback status/location = %d/%q", response.Code, response.Result().Header.Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("callback set %d cookies, want session and flow cleanup", len(cookies))
	}
	var cookie *http.Cookie
	for _, candidate := range cookies {
		if candidate.Name == defaultSessionCookieName {
			cookie = candidate
		}
	}
	if cookie == nil {
		t.Fatal("callback did not set session cookie")
	}
	if cookie.Name != defaultSessionCookieName || cookie.Value == "" || cookie.Domain != "" || cookie.Path != "/" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie is not constrained: %+v", cookie)
	}
	if provider.lastCode != "authorization-code-secret" || provider.lastVerifier == "" {
		t.Fatal("provider exchange did not receive the callback code and PKCE verifier")
	}
	if _, err := store.GetSession(cookie.Value); err != nil {
		t.Fatal(err)
	}
}

func TestMeUsesCredentialedExactOriginCORSAndExposesNoSecrets(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{identity: ProviderIdentity{
		Issuer: "https://accounts.google.com", Subject: "subject-1", Email: "person@example.com", DisplayName: "Person",
	}}
	server, _ := newTestServer(t, provider, &now)
	handler := server.Routes()
	state, flowCookie := loginState(t, handler)
	provider.identity.Nonce = provider.authorization.nonce
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=authorization-code-secret", nil)
	callback.AddCookie(flowCookie)
	callbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackResponse, callback)
	var sessionCookie *http.Cookie
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == defaultSessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("callback did not set session cookie")
	}

	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.Header.Set("Origin", "https://game.example.test")
	me.AddCookie(sessionCookie)
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, me)
	if meResponse.Code != http.StatusOK || meResponse.Header().Get("Access-Control-Allow-Origin") != "https://game.example.test" || meResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("authenticated response status/CORS = %d/%q/%q", meResponse.Code, meResponse.Header().Get("Access-Control-Allow-Origin"), meResponse.Header().Get("Access-Control-Allow-Credentials"))
	}
	responseText := meResponse.Body.String()
	var body map[string]any
	if err := json.Unmarshal([]byte(responseText), &body); err != nil {
		t.Fatal(err)
	}
	if body["email"] != "person@example.com" || body["display_name"] != "Person" || body["id"] == nil {
		t.Fatalf("unexpected current-user JSON: %#v", body)
	}
	for _, secret := range []string{"authorization-code-secret", sessionCookie.Value, "client-secret", "access-token", "refresh-token", "id-token"} {
		if strings.Contains(responseText, secret) {
			t.Fatalf("current-user response leaked %q", secret)
		}
	}

	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != http.StatusUnauthorized || !strings.HasPrefix(unauthenticatedResponse.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unauthenticated response status/content-type = %d/%q", unauthenticatedResponse.Code, unauthenticatedResponse.Header().Get("Content-Type"))
	}

	foreign := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	foreign.Header.Set("Origin", "https://evil.example.test")
	foreignResponse := httptest.NewRecorder()
	handler.ServeHTTP(foreignResponse, foreign)
	if foreignResponse.Header().Get("Access-Control-Allow-Origin") != "" || strings.Contains(foreignResponse.Header().Get("Access-Control-Allow-Origin"), "*") {
		t.Fatal("foreign origin received permissive CORS")
	}
}

func TestAPIOnlyUsesJSONExceptOAuthRedirects(t *testing.T) {
	now := time.Now().UTC()
	provider := &fakeProvider{exchangeErr: errors.New("provider token secret")}
	server, _ := newTestServer(t, provider, &now)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if response.Code != http.StatusUnauthorized || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("API error status/content-type = %d/%q", response.Code, response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), "<html") {
		t.Fatal("API error returned HTML")
	}
}

func TestInjectedFrontendFallbackUsesSharedMiddlewareAndReservedPathsStayJSON(t *testing.T) {
	now := time.Now().UTC()
	server, _ := newTestServer(t, &fakeProvider{}, &now)
	var fallbackPath, fallbackRequestID string
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackPath = r.URL.Path
		fallbackRequestID = requestID(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html>"))
	})
	handler := server.Handler(fallback)

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer

	frontendRequest := httptest.NewRequest(http.MethodGet, "/play/room-1", nil)
	frontendRequest.Header.Set("Origin", "https://game.example.test")
	frontendRequest.Header.Set("X-Request-ID", "frontend-request")
	frontendResponse := httptest.NewRecorder()
	handler.ServeHTTP(frontendResponse, frontendRequest)

	apiRequest := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	apiRequest.Header.Set("Origin", "https://game.example.test")
	apiRequest.Header.Set("X-Request-ID", "api-request")
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)

	authRequest := httptest.NewRequest(http.MethodGet, "/auth/unknown", nil)
	authRequest.Header.Set("X-Request-ID", "auth-request")
	authResponse := httptest.NewRecorder()
	handler.ServeHTTP(authResponse, authRequest)

	_ = writer.Close()
	os.Stdout = oldStdout
	logOutput, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	if frontendResponse.Code != http.StatusOK || frontendResponse.Body.String() != "<!doctype html>" || frontendResponse.Header().Get("X-Request-ID") != "frontend-request" {
		t.Fatalf("frontend fallback response = %d/%q/request-id=%q", frontendResponse.Code, frontendResponse.Body.String(), frontendResponse.Header().Get("X-Request-ID"))
	}
	if fallbackPath != "/play/room-1" || fallbackRequestID != "frontend-request" {
		t.Fatalf("fallback request = path %q/request-id %q", fallbackPath, fallbackRequestID)
	}
	if frontendResponse.Header().Get("Access-Control-Allow-Origin") != "https://game.example.test" || frontendResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("frontend fallback CORS = %q/%q", frontendResponse.Header().Get("Access-Control-Allow-Origin"), frontendResponse.Header().Get("Access-Control-Allow-Credentials"))
	}
	for name, response := range map[string]*httptest.ResponseRecorder{"api": apiResponse, "auth": authResponse} {
		if response.Code != http.StatusNotFound || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("unknown %s response = %d/content-type=%q", name, response.Code, response.Header().Get("Content-Type"))
		}
		if strings.Contains(response.Body.String(), "<!doctype html>") {
			t.Fatalf("unknown %s path reached frontend fallback", name)
		}
	}
	text := string(logOutput)
	for _, requestID := range []string{"frontend-request", "api-request", "auth-request"} {
		if !strings.Contains(text, "request_id="+requestID) {
			t.Fatalf("access log lacks request ID %q: %q", requestID, text)
		}
	}
	for _, credential := range []string{"authorization-code-secret", "session-secret", "client-secret", "access-token", "refresh-token", "id-token"} {
		if strings.Contains(text, credential) {
			t.Fatalf("access log leaked credential %q: %q", credential, text)
		}
	}
}

func TestMeAndRestReturnAPContractAndUseServerState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-1", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"}
	handler := server.Routes()

	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.Header.Set("Origin", "https://game.example.test")
	me.Header.Set("X-Request-ID", "me-request")
	me.AddCookie(cookie)
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, me)
	if meResponse.Code != http.StatusOK || meResponse.Header().Get("X-Request-ID") != "me-request" {
		t.Fatalf("GET /api/me status/request ID = %d/%q", meResponse.Code, meResponse.Header().Get("X-Request-ID"))
	}
	var meBody map[string]any
	if err := json.Unmarshal(meResponse.Body.Bytes(), &meBody); err != nil {
		t.Fatal(err)
	}
	if len(meBody) != 20 || meBody["id"] != float64(identity.ID) || meBody["display_name"] != "Person" || meBody["email"] != "person@example.com" || meBody["ap"] != float64(maxAP) {
		t.Fatalf("GET /api/me JSON = %#v", meBody)
	}
	if actions, ok := meBody["available_actions"].([]any); !ok || !reflect.DeepEqual(actions, []any{"rest", "move"}) {
		t.Fatalf("available actions = %#v, want rest and move only", meBody["available_actions"])
	}
	definitions, ok := meBody["building_extension_definitions"].([]any)
	if !ok || len(definitions) != 0 {
		t.Fatalf("extension definitions = %#v", meBody["building_extension_definitions"])
	}
	if meBody["carried_weight"] != float64(0) || meBody["movement_weight_threshold"] != float64(movementWeightThreshold) {
		t.Fatalf("GET /api/me carrying weight = %#v/%#v, want 0/%d", meBody["carried_weight"], meBody["movement_weight_threshold"], movementWeightThreshold)
	}
	if groundItems, ok := meBody["ground_items"].([]any); !ok || len(groundItems) != 0 {
		t.Fatalf("GET /api/me ground items = %#v", meBody["ground_items"])
	}
	if groundResources, ok := meBody["ground_resources"].([]any); !ok || len(groundResources) != 0 {
		t.Fatalf("GET /api/me ground resources = %#v", meBody["ground_resources"])
	}
	buildingRecipes, ok := meBody["building_recipes"].([]any)
	if !ok || len(buildingRecipes) != 0 {
		t.Fatalf("GET /api/me building recipes = %#v", meBody["building_recipes"])
	}
	buildings, ok := meBody["buildings"].([]any)
	if !ok || len(buildings) != 0 {
		t.Fatalf("GET /api/me buildings = %#v", meBody["buildings"])
	}
	recipes, ok := meBody["crafting_recipes"].([]any)
	if !ok || len(recipes) != 0 {
		t.Fatalf("GET /api/me crafting recipes = %#v", meBody["crafting_recipes"])
	}
	resources := responseResourceQuantities(t, meBody)
	if len(resources) != 8 {
		t.Fatalf("GET /api/me resources = %#v", resources)
	}
	for _, resourceID := range []string{"food", "wood", "stone", "metal", "fiber", "hide", "medicinal", "arcane"} {
		if resources[resourceID] != 0 {
			t.Fatalf("GET /api/me resource %s = %v, want 0", resourceID, resources[resourceID])
		}
	}
	location, ok := meBody["location"].(map[string]any)
	if !ok || location["id"] != "camp" || location["display_name"] != "Camp" {
		t.Fatalf("GET /api/me location = %#v", meBody["location"])
	}
	routes, ok := meBody["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("GET /api/me routes = %#v", meBody["routes"])
	}
	if route, ok := routes[0].(map[string]any); !ok || route["origin_id"] != "camp" || route["destination_id"] != "forest_edge" || route["ap_cost"] != float64(20) {
		t.Fatalf("GET /api/me route = %#v", routes[0])
	}
	if inventory, ok := meBody["inventory"].([]any); !ok || len(inventory) != 0 {
		t.Fatalf("GET /api/me inventory = %#v", meBody["inventory"])
	}
	if meBody["gathering_option"] != nil {
		t.Fatalf("GET /api/me camp gathering option = %#v", meBody["gathering_option"])
	}
	if meBody["conversion_option"] != nil {
		t.Fatalf("GET /api/me camp conversion option = %#v", meBody["conversion_option"])
	}

	rest := httptest.NewRequest(http.MethodPost, "/api/actions/rest", strings.NewReader(`{"ap":0,"time":"9999-01-01T00:00:00Z"}`))
	rest.Header.Set("Origin", "https://game.example.test")
	rest.Header.Set("Content-Type", "application/json")
	rest.AddCookie(cookie)
	restResponse := httptest.NewRecorder()
	handler.ServeHTTP(restResponse, rest)
	if restResponse.Code != http.StatusOK {
		t.Fatalf("POST /api/actions/rest status = %d: %s", restResponse.Code, restResponse.Body.String())
	}
	var restBody map[string]any
	if err := json.Unmarshal(restResponse.Body.Bytes(), &restBody); err != nil {
		t.Fatal(err)
	}
	if len(restBody) != 17 || restBody["ap"] != float64(maxAP-1) {
		t.Fatalf("POST /api/actions/rest JSON = %#v", restBody)
	}
	if ap, err := store.GetAP(identity.ID); err != nil || ap != maxAP-1 {
		t.Fatalf("server did not persist rest result: ap=%d err=%v", ap, err)
	}

	unauthenticated := httptest.NewRequest(http.MethodPost, "/api/actions/rest", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated rest status = %d", unauthenticatedResponse.Code)
	}
}

func TestBuildingAPIUsesBackendRecipeAndAuthoritativeState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "building-api", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10); INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	handler := server.Routes()
	request := httptest.NewRequest(http.MethodPost, "/api/actions/build", strings.NewReader(`{"recipe_id":"building_lv1","required_ap":1}`))
	request.Header.Set("X-Request-ID", "build-api")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	var logOutput string
	logOutput = captureStdout(t, func() { handler.ServeHTTP(response, request) })
	if response.Code != http.StatusBadRequest || strings.Contains(logOutput, `"recipe_id"`) {
		t.Fatalf("build whitelist status/log = %d/%q", response.Code, logOutput)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/build", strings.NewReader(`{"recipe_id":"building_lv1"}`))
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("build status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	buildings, ok := body["buildings"].([]any)
	if !ok || len(buildings) != 1 {
		t.Fatalf("buildings response = %#v", body["buildings"])
	}
	building := buildings[0].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(building), []string{"available_actions", "building_level", "contributed_ap", "durability_percentage", "durability_status", "extension_slot_count", "extensions", "id", "owner", "recipe", "required_ap", "status"}) || building["required_ap"] != float64(60) || building["contributed_ap"] != float64(0) || building["status"] != "under_construction" || building["durability_status"] != nil || building["durability_percentage"] != nil {
		t.Fatalf("building response = %#v", building)
	}
	recipe := building["recipe"].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(recipe), []string{"display_name", "id"}) || recipe["id"] != "building_lv1" || recipe["display_name"] != "Building Lv1" {
		t.Fatalf("building recipe response = %#v", recipe)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(`{"building_id":1,"ap":10,"extra":true}`))
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("contribution extra field status = %d", response.Code)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	buildings = body["buildings"].([]any)
	if buildings[0].(map[string]any)["contributed_ap"] != float64(0) {
		t.Fatalf("rejected contribution changed state = %#v", buildings[0])
	}
	for _, input := range []string{`{}`, `{"building_id":1,"ap":0}`, `{"building_id":1,"ap":-1}`} {
		request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(input))
		request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
		failure := httptest.NewRecorder()
		handler.ServeHTTP(failure, request)
		if failure.Code != http.StatusBadRequest {
			t.Fatalf("malformed contribution %s status = %d", input, failure.Code)
		}
		var failureBody map[string]any
		if err := json.Unmarshal(failure.Body.Bytes(), &failureBody); err != nil {
			t.Fatal(err)
		}
		if failureBody["buildings"].([]any)[0].(map[string]any)["contributed_ap"] != float64(0) {
			t.Fatalf("malformed contribution changed state = %#v", failureBody)
		}
	}
	if _, err := store.db.Exec(`INSERT INTO locations (id, display_name) VALUES ('remote-building-location', 'Remote')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE player_locations SET location_id = 'remote-building-location' WHERE user_id = ?`, identity.ID); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(`{"building_id":1,"ap":1}`))
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("remote contribution status = %d", response.Code)
	}
	if _, err := store.db.Exec(`UPDATE player_locations SET location_id = 'camp' WHERE user_id = ?`, identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, now.Add(time.Duration(maxAP)*time.Minute).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(`{"building_id":1,"ap":1}`))
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("insufficient AP contribution status = %d", response.Code)
	}
	if _, err := store.db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, now.Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(`{"building_id":1,"ap":100}`))
	request.Header.Set("X-Request-ID", "contribute-success")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	logOutput = captureStdout(t, func() { handler.ServeHTTP(response, request) })
	if response.Code != http.StatusOK || !strings.Contains(logOutput, "user_id=1 action=contribute-construction outcome=success building_id=1 effective_ap=60 resulting_progress=60/60 completion=completed request_id=contribute-success") {
		t.Fatalf("successful contribution status/log = %d/%q", response.Code, logOutput)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP-60) || len(body["buildings"].([]any)) != 1 || body["buildings"].([]any)[0].(map[string]any)["status"] != "completed" {
		t.Fatalf("successful contribution state = %#v", body)
	}
	for _, secret := range []string{"session-secret", "oauth-code-secret", "access-token", "refresh-token", "id-token"} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("contribution log leaked %q: %q", secret, logOutput)
		}
	}
	for _, rawInput := range []string{`{"building_id":1`, `"ap":100`} {
		if strings.Contains(logOutput, rawInput) {
			t.Fatalf("contribution log included raw input %q: %q", rawInput, logOutput)
		}
	}
	for _, input := range []string{`{"building_id":999,"ap":1}`, `{"building_id":1,"ap":1}`} {
		request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(input))
		request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
		failure := httptest.NewRecorder()
		handler.ServeHTTP(failure, request)
		if failure.Code != http.StatusBadRequest && failure.Code != http.StatusConflict {
			t.Fatalf("rejected contribution %s status = %d", input, failure.Code)
		}
		var failureBody map[string]any
		if err := json.Unmarshal(failure.Body.Bytes(), &failureBody); err != nil {
			t.Fatal(err)
		}
		failureBuilding := failureBody["buildings"].([]any)[0].(map[string]any)
		if failureBody["ap"] != float64(maxAP-60) || failureBuilding["status"] != "completed" {
			t.Fatalf("rejected contribution changed state = %#v", failureBody)
		}
		if _, ok := failureBuilding["contributed_ap"]; ok {
			t.Fatalf("completed rejected contribution exposed construction AP = %#v", failureBuilding)
		}
	}
}

type buildingAPIFixture struct {
	server   *Server
	store    *Store
	identity Identity
	now      time.Time
	cookie   *http.Cookie
}

func newBuildingAPIFixture(t *testing.T, subject string) buildingAPIFixture {
	t.Helper()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", subject, "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	sessionToken := subject + "-session-token"
	if err := store.CreateSession(identity.ID, sessionToken, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return buildingAPIFixture{server: server, store: store, identity: identity, now: now, cookie: &http.Cookie{Name: defaultSessionCookieName, Value: sessionToken}}
}

func (f buildingAPIFixture) request(method, path, body, requestID string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", requestID)
	request.AddCookie(f.cookie)
	return request
}

func buildingFromResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	buildings, ok := response["buildings"].([]any)
	if !ok || len(buildings) != 1 {
		t.Fatalf("building response = %#v", response["buildings"])
	}
	building, ok := buildings[0].(map[string]any)
	if !ok {
		t.Fatalf("building entry = %#v", buildings[0])
	}
	return building
}

func assertBuildingResponseContract(t *testing.T, building map[string]any, contributedAP float64, status string) {
	t.Helper()
	wantKeys := []string{"available_actions", "building_level", "durability_percentage", "durability_status", "extension_slot_count", "extensions", "id", "owner", "recipe", "status"}
	if status == "under_construction" {
		wantKeys = append(wantKeys, "contributed_ap", "required_ap")
		sort.Strings(wantKeys)
	}
	if !reflect.DeepEqual(sortedMapKeys(building), wantKeys) {
		t.Fatalf("building keys = %#v, want %#v", sortedMapKeys(building), wantKeys)
	}
	if building["id"] != float64(1) || building["building_level"] != float64(1) || building["status"] != status || building["extension_slot_count"] != float64(1) {
		t.Fatalf("building contract = %#v", building)
	}
	if status == "under_construction" {
		if building["required_ap"] != float64(60) || building["contributed_ap"] != contributedAP {
			t.Fatalf("under-construction AP = %#v/%#v", building["required_ap"], building["contributed_ap"])
		}
		if building["durability_status"] != nil || building["durability_percentage"] != nil {
			t.Fatalf("under-construction durability = %#v/%#v", building["durability_status"], building["durability_percentage"])
		}
	} else if building["durability_status"] != "active" || building["durability_percentage"] != float64(100) {
		t.Fatalf("completed durability = %#v/%#v", building["durability_status"], building["durability_percentage"])
	}
	if status != "under_construction" {
		for _, key := range []string{"contributed_ap", "required_ap"} {
			if _, ok := building[key]; ok {
				t.Fatalf("completed building exposes construction key %q: %#v", key, building)
			}
		}
	}
	owner, ok := building["owner"].(map[string]any)
	if !ok || !reflect.DeepEqual(sortedMapKeys(owner), []string{"display_name", "id"}) || owner["id"] != float64(1) || owner["display_name"] != "Person" {
		t.Fatalf("building owner contract = %#v", building["owner"])
	}
	recipe, ok := building["recipe"].(map[string]any)
	if !ok || !reflect.DeepEqual(sortedMapKeys(recipe), []string{"display_name", "id"}) || recipe["id"] != "building_lv1" || recipe["display_name"] != "Building Lv1" {
		t.Fatalf("building recipe contract = %#v", building["recipe"])
	}
}

func TestBuildingAPIConditionallyExposesConstructionAPForBuildingsAndExtensions(t *testing.T) {
	fixture := newBuildingAPIFixture(t, "building-api-conditional-construction")
	prepareBuilding(t, fixture, "under_construction", 12)
	buildingID := preparedBuildingID(t, fixture)
	if _, err := fixture.store.db.Exec(`INSERT INTO building_extensions (building_id, slot_index, definition_id, display_name, tier, required_ap, contributed_ap, status) VALUES (?, 0, 'sawmill_t1', 'Sawmill T1', 1, 30, 12, 'under_construction')`, buildingID); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}

	getState := func(requestID string) (map[string]any, string) {
		t.Helper()
		response := httptest.NewRecorder()
		logOutput := captureStdout(t, func() {
			fixture.server.Routes().ServeHTTP(response, fixture.request(http.MethodGet, "/api/me", "", requestID))
		})
		if response.Code != http.StatusOK {
			t.Fatalf("GET /api/me status = %d: %s", response.Code, response.Body.String())
		}
		return buildingFromResponse(t, response.Body.Bytes()), logOutput
	}

	building, logOutput := getState("building-api-conditional-under-construction")
	assertBuildingResponseContract(t, building, 12, "under_construction")
	extensions, ok := building["extensions"].([]any)
	if !ok || len(extensions) != 1 {
		t.Fatalf("under-construction extensions = %#v", building["extensions"])
	}
	extension := extensions[0].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(extension), []string{"available_actions", "contributed_ap", "definition_id", "display_name", "id", "required_ap", "slot_index", "status", "tier"}) || extension["required_ap"] != float64(30) || extension["contributed_ap"] != float64(12) || extension["status"] != "under_construction" {
		t.Fatalf("under-construction extension contract = %#v", extension)
	}
	if !strings.Contains(logOutput, "user_id=1 action=GET /api/me outcome=success") {
		t.Fatalf("state response log = %q", logOutput)
	}
	after, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("response shaping changed authoritative state: before=%+v after=%+v", before, after)
	}

	if _, err := fixture.store.db.Exec(`UPDATE buildings SET status = 'completed', contributed_ap = 60, durability_expires_at = ? WHERE id = ?`, fixture.now.Add(time.Duration(buildingDefaultDurabilitySeconds)*time.Second).Unix(), buildingID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.db.Exec(`UPDATE building_extensions SET status = 'completed', contributed_ap = 30 WHERE building_id = ?`, buildingID); err != nil {
		t.Fatal(err)
	}
	completedBefore, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	building, logOutput = getState("building-api-conditional-completed")
	assertBuildingResponseContract(t, building, 60, "completed")
	extensions, ok = building["extensions"].([]any)
	if !ok || len(extensions) != 1 {
		t.Fatalf("completed extensions = %#v", building["extensions"])
	}
	extension = extensions[0].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(extension), []string{"available_actions", "definition_id", "display_name", "id", "slot_index", "status", "tier"}) || extension["status"] != "completed" {
		t.Fatalf("completed extension contract = %#v", extension)
	}
	for _, key := range []string{"contributed_ap", "required_ap", "percentage"} {
		if _, ok := extension[key]; ok {
			t.Fatalf("completed extension exposes construction key %q: %#v", key, extension)
		}
	}
	if !strings.Contains(logOutput, "user_id=1 action=building_durability_calculation outcome=success") || !strings.Contains(logOutput, "user_id=1 action=GET /api/me outcome=success") {
		t.Fatalf("completed state response log = %q", logOutput)
	}
	completedAfter, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(completedBefore, completedAfter) {
		t.Fatalf("completed response shaping changed authoritative state: before=%+v after=%+v", completedBefore, completedAfter)
	}
}

func assertUnchangedBuildingState(t *testing.T, before PlayerState, after PlayerState) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected action changed authoritative state: before=%+v after=%+v", before, after)
	}
}

func prepareBuilding(t *testing.T, fixture buildingAPIFixture, status string, contributedAP int) {
	t.Helper()
	if _, err := fixture.store.db.Exec(`INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count) VALUES (?, 'camp', 'building_lv1', 'Building Lv1', 1, 60, ?, ?, 1)`, fixture.identity.ID, contributedAP, status); err != nil {
		t.Fatal(err)
	}
}

func preparedBuildingID(t *testing.T, fixture buildingAPIFixture) int64 {
	t.Helper()
	var buildingID int64
	if err := fixture.store.db.QueryRow(`SELECT id FROM buildings WHERE owner_id = ?`, fixture.identity.ID).Scan(&buildingID); err != nil {
		t.Fatal(err)
	}
	return buildingID
}

func TestBuildingAPIReturnsExactRecipeAndBuildingContracts(t *testing.T) {
	fixture := newBuildingAPIFixture(t, "building-api-contract")
	if _, err := fixture.store.db.Exec(`
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10);
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, fixture.identity.ID, fixture.identity.ID); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := fixture.request(http.MethodGet, "/api/me", "", "building-contract-me")
	fixture.server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/me status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	recipes, ok := body["building_recipes"].([]any)
	if !ok || len(recipes) != 1 {
		t.Fatalf("building recipes = %#v", body["building_recipes"])
	}
	recipe := recipes[0].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(recipe), []string{"building_level", "display_name", "extension_slot_count", "id", "item_inputs", "required_ap", "resource_inputs"}) || recipe["id"] != "building_lv1" || recipe["display_name"] != "Building Lv1" || recipe["building_level"] != float64(1) || recipe["required_ap"] != float64(60) || recipe["extension_slot_count"] != float64(1) {
		t.Fatalf("recipe contract = %#v", recipe)
	}
	resourceInputs, ok := recipe["resource_inputs"].([]any)
	if !ok || len(resourceInputs) != 1 {
		t.Fatalf("recipe resource inputs = %#v", recipe["resource_inputs"])
	}
	resourceInput := resourceInputs[0].(map[string]any)
	resource := resourceInput["resource"].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(resourceInput), []string{"quantity", "resource"}) || !reflect.DeepEqual(sortedMapKeys(resource), []string{"display_name", "id"}) || resource["id"] != "wood" || resource["display_name"] != "Wood" || resourceInput["quantity"] != float64(10) {
		t.Fatalf("recipe resource input = %#v", resourceInput)
	}
	itemInputs, ok := recipe["item_inputs"].([]any)
	if !ok || len(itemInputs) != 1 {
		t.Fatalf("recipe item inputs = %#v", recipe["item_inputs"])
	}
	itemInput := itemInputs[0].(map[string]any)
	item := itemInput["item"].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(itemInput), []string{"item", "quantity"}) || !reflect.DeepEqual(sortedMapKeys(item), []string{"display_name", "id"}) || item["id"] != "wood_component" || item["display_name"] != "Wood Component" || itemInput["quantity"] != float64(1) {
		t.Fatalf("recipe item input = %#v", itemInput)
	}

	buildRequest := fixture.request(http.MethodPost, "/api/actions/build", `{"recipe_id":"building_lv1"}`, "building-contract-build")
	buildResponse := httptest.NewRecorder()
	buildLog := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(buildResponse, buildRequest) })
	if buildResponse.Code != http.StatusOK || !strings.Contains(buildLog, "user_id="+strconv.FormatInt(fixture.identity.ID, 10)+" action=build outcome=success request_id=building-contract-build") {
		t.Fatalf("build status/log = %d/%q", buildResponse.Code, buildLog)
	}
	assertBuildingResponseContract(t, buildingFromResponse(t, buildResponse.Body.Bytes()), 0, "under_construction")
}

func TestBuildAPIRejectsEveryInvalidRequestWithoutStateChangesOrLogLeak(t *testing.T) {
	tests := []struct {
		name, body, reason string
		status             int
	}{
		{"invalid JSON", `{`, buildReasonInvalidJSON, http.StatusBadRequest},
		{"unknown field", `{"unexpected":"value"}`, buildReasonUnknownField, http.StatusBadRequest},
		{"duplicate recipe", `{"recipe_id":"building_lv1","recipe_id":"building_lv1"}`, buildReasonDuplicate, http.StatusBadRequest},
		{"extra JSON value", `{"recipe_id":"building_lv1"}{}`, buildReasonExtraValue, http.StatusBadRequest},
		{"missing recipe", `{}`, buildReasonMissingRecipe, http.StatusBadRequest},
		{"blank recipe", `{"recipe_id":"  "}`, buildReasonInvalidRecipe, http.StatusBadRequest},
		{"invalid recipe type", `{"recipe_id":1}`, buildReasonInvalidRecipe, http.StatusBadRequest},
		{"unknown recipe", `{"recipe_id":"unknown"}`, buildReasonUnknownRecipe, http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBuildingAPIFixture(t, "building-api-reject-"+strings.ReplaceAll(test.name, " ", "-"))
			before, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "build-reject-" + strings.ReplaceAll(test.name, " ", "-")
			request := fixture.request(http.MethodPost, "/api/actions/build", test.body, requestID)
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
			if response.Code != test.status || !strings.Contains(logOutput, "user_id=1 action=build outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("rejection status/log = %d/%q", response.Code, logOutput)
			}
			after, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchangedBuildingState(t, before, after)
		})
	}

	for _, test := range []struct {
		name, reason string
		prepare      func(buildingAPIFixture)
	}{
		{"insufficient resource", buildReasonInsufficientResource, func(f buildingAPIFixture) {
			if _, err := f.store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, f.identity.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"insufficient item", buildReasonInsufficientItem, func(f buildingAPIFixture) {
			if _, err := f.store.db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)`, f.identity.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"occupied owner slot", buildReasonOccupied, func(f buildingAPIFixture) {
			if _, err := f.store.db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10); INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, f.identity.ID, f.identity.ID); err != nil {
				t.Fatal(err)
			}
			prepareBuilding(t, f, "under_construction", 0)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBuildingAPIFixture(t, "building-api-reject-"+strings.ReplaceAll(test.name, " ", "-"))
			test.prepare(fixture)
			before, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "build-reject-" + strings.ReplaceAll(test.name, " ", "-")
			request := fixture.request(http.MethodPost, "/api/actions/build", `{"recipe_id":"building_lv1"}`, requestID)
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusConflict || !strings.Contains(logOutput, "user_id=1 action=build outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("rejection status/log = %d/%q", response.Code, logOutput)
			}
			if test.name == "insufficient resource" {
				var body map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["error"] != ErrInsufficientResource.Error() {
					t.Fatalf("insufficient resource error = %#v, want %q", body["error"], ErrInsufficientResource.Error())
				}
			}
			after, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchangedBuildingState(t, before, after)
		})
	}

	fixture := newBuildingAPIFixture(t, "building-api-sensitive-rejection")
	before, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	sensitiveBody := `{"credentials":"credential-sentinel","session":"session-sentinel","oauth":"oauth-sentinel","raw_input":"raw-input-sentinel"}`
	request := fixture.request(http.MethodPost, "/api/actions/build", sensitiveBody, "build-sensitive-rejection")
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusBadRequest || !strings.Contains(logOutput, "user_id=1 action=build outcome=error reason=unknown_field request_id=build-sensitive-rejection") {
		t.Fatalf("sensitive rejection status/log = %d/%q", response.Code, logOutput)
	}
	for _, sentinel := range []string{"credential-sentinel", "session-sentinel", "oauth-sentinel", "raw-input-sentinel"} {
		if strings.Contains(logOutput, sentinel) {
			t.Fatalf("rejection log leaked %q: %q", sentinel, logOutput)
		}
	}
	after, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertUnchangedBuildingState(t, before, after)
}

func TestContributeConstructionAPIValidatesRequestsAndPreservesState(t *testing.T) {
	tests := []struct {
		name, body, reason string
	}{
		{"invalid JSON", `{`, contributeReasonInvalidJSON},
		{"unknown field", `{"unexpected":1}`, contributeReasonUnknownField},
		{"duplicate building", `{"building_id":1,"building_id":1,"ap":1}`, contributeReasonDuplicate},
		{"duplicate AP", `{"building_id":1,"ap":1,"ap":1}`, contributeReasonDuplicate},
		{"extra JSON value", `{"building_id":1,"ap":1}{}`, contributeReasonExtraValue},
		{"missing building", `{"ap":1}`, contributeReasonMissingBuilding},
		{"building string", `{"building_id":"1","ap":1}`, contributeReasonInvalidBuilding},
		{"building zero", `{"building_id":0,"ap":1}`, contributeReasonInvalidBuilding},
		{"building negative", `{"building_id":-1,"ap":1}`, contributeReasonInvalidBuilding},
		{"missing AP", `{"building_id":1}`, contributeReasonMissingAP},
		{"AP string", `{"building_id":1,"ap":"1"}`, contributeReasonInvalidAP},
		{"AP zero", `{"building_id":1,"ap":0}`, contributeReasonInvalidAP},
		{"AP negative", `{"building_id":1,"ap":-1}`, contributeReasonInvalidAP},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBuildingAPIFixture(t, "contribution-api-decode-"+strings.ReplaceAll(test.name, " ", "-"))
			prepareBuilding(t, fixture, "under_construction", 0)
			before, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "contribution-decode-" + strings.ReplaceAll(test.name, " ", "-")
			request := fixture.request(http.MethodPost, "/api/actions/contribute-construction", test.body, requestID)
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusBadRequest || !strings.Contains(logOutput, "user_id=1 action=contribute-construction outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("rejection status/log = %d/%q", response.Code, logOutput)
			}
			after, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchangedBuildingState(t, before, after)
		})
	}

	fixture := newBuildingAPIFixture(t, "contribution-api-sensitive-rejection")
	prepareBuilding(t, fixture, "under_construction", 0)
	before, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	sensitiveBody := `{"credentials":"credential-sentinel","session":"session-sentinel","oauth":"oauth-sentinel","raw_input":"raw-input-sentinel"}`
	request := fixture.request(http.MethodPost, "/api/actions/contribute-construction", sensitiveBody, "contribution-sensitive-rejection")
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusBadRequest || !strings.Contains(logOutput, "user_id="+strconv.FormatInt(fixture.identity.ID, 10)+" action=contribute-construction outcome=error reason=unknown_field request_id=contribution-sensitive-rejection") {
		t.Fatalf("sensitive rejection status/log = %d/%q", response.Code, logOutput)
	}
	for _, sentinel := range []string{"credential-sentinel", "session-sentinel", "oauth-sentinel", "raw-input-sentinel"} {
		if strings.Contains(logOutput, sentinel) {
			t.Fatalf("rejection log leaked %q: %q", sentinel, logOutput)
		}
	}
	after, err := fixture.store.GetPlayerState(fixture.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertUnchangedBuildingState(t, before, after)
}

func TestContributeConstructionAPIReturnsExactSuccessAndRejectsDomainFailures(t *testing.T) {
	fixture := newBuildingAPIFixture(t, "contribution-api-success")
	prepareBuilding(t, fixture, "under_construction", 0)
	request := fixture.request(http.MethodPost, "/api/actions/contribute-construction", `{"building_id":1,"ap":60}`, "contribution-success-contract")
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK || !strings.Contains(logOutput, "user_id="+strconv.FormatInt(fixture.identity.ID, 10)+" action=contribute-construction outcome=success request_id=contribution-success-contract") {
		t.Fatalf("successful contribution status/log = %d/%q", response.Code, logOutput)
	}
	building := buildingFromResponse(t, response.Body.Bytes())
	assertBuildingResponseContract(t, building, 60, "completed")
}

func TestContributeConstructionAPIRejectsDomainFailuresWithoutStateChangesOrLogLeak(t *testing.T) {
	for _, test := range []struct {
		name, body, reason string
		status             int
		prepare            func(buildingAPIFixture)
	}{
		{"unknown target", `{"building_id":999,"ap":1}`, contributeReasonUnknownBuilding, http.StatusBadRequest, nil},
		{"remote Location", `{"building_id":1,"ap":1}`, contributeReasonRemote, http.StatusConflict, func(f buildingAPIFixture) {
			if _, err := f.store.db.Exec(`INSERT INTO locations (id, display_name) VALUES ('remote-building-location', 'Remote')`); err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.db.Exec(`UPDATE player_locations SET location_id = 'remote-building-location' WHERE user_id = ?`, f.identity.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"completed Building", `{"building_id":1,"ap":1}`, contributeReasonCompleted, http.StatusConflict, nil},
		{"insufficient AP", `{"building_id":1,"ap":1}`, contributeReasonInsufficientAP, http.StatusConflict, func(f buildingAPIFixture) {
			if _, err := f.store.db.Exec(`UPDATE buildings SET status = 'under_construction', contributed_ap = 0`); err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, f.now.Add(maxAP*time.Minute).Unix(), f.identity.ID); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseFixture := newBuildingAPIFixture(t, "contribution-api-domain-"+strings.ReplaceAll(test.name, " ", "-"))
			prepareBuilding(t, caseFixture, "under_construction", 0)
			buildingID := preparedBuildingID(t, caseFixture)
			if test.name == "completed Building" {
				if _, err := caseFixture.store.db.Exec(`UPDATE buildings SET status = 'completed', contributed_ap = 60`); err != nil {
					t.Fatal(err)
				}
			}
			if test.prepare != nil {
				test.prepare(caseFixture)
			}
			before, err := caseFixture.store.GetPlayerState(caseFixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "contribution-domain-" + strings.ReplaceAll(test.name, " ", "-")
			body := strings.ReplaceAll(test.body, `"building_id":1`, `"building_id":`+strconv.FormatInt(buildingID, 10))
			request := caseFixture.request(http.MethodPost, "/api/actions/contribute-construction", body, requestID)
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { caseFixture.server.Routes().ServeHTTP(response, request) })
			if response.Code != test.status || !strings.Contains(logOutput, "user_id="+strconv.FormatInt(caseFixture.identity.ID, 10)+" action=contribute-construction outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("rejection status/log = %d/%q", response.Code, logOutput)
			}
			if test.name == "unknown target" {
				var body map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["error"] != ErrBuildingNotFound.Error() {
					t.Fatalf("unknown building error = %#v, want %q", body["error"], ErrBuildingNotFound.Error())
				}
			}
			after, err := caseFixture.store.GetPlayerState(caseFixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchangedBuildingState(t, before, after)
		})
	}
}

func prepareCompletedBuilding(t *testing.T, fixture buildingAPIFixture, expiresAt time.Time) int64 {
	t.Helper()
	prepareBuilding(t, fixture, "completed", 60)
	buildingID := preparedBuildingID(t, fixture)
	if _, err := fixture.store.db.Exec(`UPDATE buildings SET durability_expires_at = ? WHERE id = ?`, expiresAt.Unix(), buildingID); err != nil {
		t.Fatal(err)
	}
	return buildingID
}

func prepareExtensionActionFixture(t *testing.T, subject string) (buildingAPIFixture, int64, int64) {
	t.Helper()
	fixture := newBuildingAPIFixture(t, subject)
	buildingID := prepareCompletedBuilding(t, fixture, fixture.now.Add(time.Hour))
	if _, err := fixture.store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'sawmill_package_t1', 1)`, fixture.identity.ID); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.InstallExtension(fixture.identity.ID, buildingID, 0, "sawmill_t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Buildings) != 1 || len(state.Buildings[0].Extensions) != 1 {
		t.Fatalf("installed extension state = %#v", state.Buildings)
	}
	return fixture, buildingID, state.Buildings[0].Extensions[0].ID
}

func TestInstallExtensionAPILogUsesRequestBuildingAndCreatedExtension(t *testing.T) {
	fixture := newBuildingAPIFixture(t, "extension-api-install-log")
	buildingID := prepareCompletedBuilding(t, fixture, fixture.now.Add(time.Hour))
	if _, err := fixture.store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'sawmill_package_t1', 1)`, fixture.identity.ID); err != nil {
		t.Fatal(err)
	}
	requestID := "extension-install-success"
	body := `{"building_id":` + strconv.FormatInt(buildingID, 10) + `,"slot_index":0,"definition_id":"sawmill_t1"}`
	request := fixture.request(http.MethodPost, "/api/actions/install-extension", body, requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	want := "user_id=1 action=install-extension building_id=" + strconv.FormatInt(buildingID, 10) + " extension_id=1 ap=0 outcome=success request_id=" + requestID
	if response.Code != http.StatusOK || !strings.Contains(logOutput, want) {
		t.Fatalf("install status/log = %d/%q", response.Code, logOutput)
	}
}

func TestContributeExtensionAPILogUsesRequestValuesAndParentBuilding(t *testing.T) {
	fixture, buildingID, extensionID := prepareExtensionActionFixture(t, "extension-api-contribute-log")
	requestID := "extension-contribute-success"
	body := `{"extension_id":1,"ap":10}`
	body = strings.Replace(body, "1", strconv.FormatInt(extensionID, 10), 1)
	request := fixture.request(http.MethodPost, "/api/actions/contribute-extension-construction", body, requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	want := "user_id=1 action=contribute-extension-construction building_id=" + strconv.FormatInt(buildingID, 10) + " extension_id=" + strconv.FormatInt(extensionID, 10) + " ap=10 outcome=success request_id=" + requestID
	if response.Code != http.StatusOK || !strings.Contains(logOutput, want) {
		t.Fatalf("contribute status/log = %d/%q", response.Code, logOutput)
	}
}

func TestContributeExtensionAPILogUsesEffectiveAPWhenRequestIsClamped(t *testing.T) {
	fixture, buildingID, extensionID := prepareExtensionActionFixture(t, "extension-api-contribute-clamped-log")
	if _, err := fixture.store.db.Exec(`UPDATE building_extensions SET contributed_ap = 25 WHERE id = ?`, extensionID); err != nil {
		t.Fatal(err)
	}
	requestID := "extension-contribute-clamped-success"
	body := `{"extension_id":1,"ap":30}`
	body = strings.Replace(body, "1", strconv.FormatInt(extensionID, 10), 1)
	request := fixture.request(http.MethodPost, "/api/actions/contribute-extension-construction", body, requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	want := "user_id=1 action=contribute-extension-construction building_id=" + strconv.FormatInt(buildingID, 10) + " extension_id=" + strconv.FormatInt(extensionID, 10) + " ap=5 outcome=success request_id=" + requestID
	if response.Code != http.StatusOK || !strings.Contains(logOutput, want) || !strings.Contains(logOutput, "requested_ap=30 effective_ap=5 resulting_progress=30/30 status=completed") {
		t.Fatalf("clamped contribution status/log = %d/%q", response.Code, logOutput)
	}
}

func TestRemoveExtensionAPILogUsesRequestValuesAndParentBuilding(t *testing.T) {
	fixture, buildingID, extensionID := prepareExtensionActionFixture(t, "extension-api-remove-log")
	requestID := "extension-remove-success"
	body := `{"extension_id":1}`
	body = strings.Replace(body, "1", strconv.FormatInt(extensionID, 10), 1)
	request := fixture.request(http.MethodPost, "/api/actions/remove-extension", body, requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	want := "user_id=1 action=remove-extension building_id=" + strconv.FormatInt(buildingID, 10) + " extension_id=" + strconv.FormatInt(extensionID, 10) + " ap=0 outcome=success request_id=" + requestID
	if response.Code != http.StatusOK || !strings.Contains(logOutput, want) {
		t.Fatalf("remove status/log = %d/%q", response.Code, logOutput)
	}
}

func TestContributeExtensionAPILogUsesAuthoritativeBuildingOnDomainFailure(t *testing.T) {
	fixture, buildingID, extensionID := prepareExtensionActionFixture(t, "extension-api-domain-log")
	if _, err := fixture.store.db.Exec(`INSERT INTO locations (id, display_name) VALUES ('remote-extension-location', 'Remote')`); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.db.Exec(`UPDATE player_locations SET location_id = 'remote-extension-location' WHERE user_id = ?`, fixture.identity.ID); err != nil {
		t.Fatal(err)
	}
	requestID := "extension-contribute-domain-failure"
	body := `{"extension_id":1,"ap":1}`
	body = strings.Replace(body, "1", strconv.FormatInt(extensionID, 10), 1)
	request := fixture.request(http.MethodPost, "/api/actions/contribute-extension-construction", body, requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	want := "user_id=1 action=contribute-extension-construction building_id=" + strconv.FormatInt(buildingID, 10) + " extension_id=" + strconv.FormatInt(extensionID, 10) + " ap=0 outcome=error request_id=" + requestID
	if response.Code != http.StatusConflict || !strings.Contains(logOutput, want) || !strings.Contains(logOutput, "requested_ap=1 effective_ap=0 resulting_progress=0/30 status=under_construction") {
		t.Fatalf("domain failure status/log = %d/%q", response.Code, logOutput)
	}
}

func TestBuildingAPIExposesDurabilityStatesAndOmitsDestroyedBuildings(t *testing.T) {
	tests := []struct {
		name           string
		expiresAt      *time.Time
		wantStatus     any
		wantPercentage any
		wantBuildings  int
	}{
		{name: "under construction", wantBuildings: 1},
		{name: "active", expiresAt: func() *time.Time { value := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC); return &value }(), wantStatus: "active", wantPercentage: float64(100), wantBuildings: 1},
		{name: "disabled", expiresAt: func() *time.Time { value := time.Date(2026, 8, 25, 11, 59, 59, 0, time.UTC); return &value }(), wantStatus: "disabled", wantPercentage: float64(0), wantBuildings: 1},
		{name: "destroyed", expiresAt: func() *time.Time { value := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC); return &value }(), wantBuildings: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBuildingAPIFixture(t, "building-durability-api-"+strings.ReplaceAll(test.name, " ", "-"))
			if test.expiresAt == nil {
				prepareBuilding(t, fixture, "under_construction", 0)
			} else {
				prepareCompletedBuilding(t, fixture, *test.expiresAt)
			}
			request := fixture.request(http.MethodGet, "/api/me", "", "building-durability-"+strings.ReplaceAll(test.name, " ", "-"))
			response := httptest.NewRecorder()
			fixture.server.Routes().ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("GET /api/me status = %d: %s", response.Code, response.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			buildings, ok := body["buildings"].([]any)
			if !ok || len(buildings) != test.wantBuildings {
				t.Fatalf("buildings = %#v, want %d entries", body["buildings"], test.wantBuildings)
			}
			if test.wantBuildings == 0 {
				return
			}
			building, ok := buildings[0].(map[string]any)
			if !ok {
				t.Fatalf("building = %#v", buildings[0])
			}
			if building["durability_status"] != test.wantStatus || building["durability_percentage"] != test.wantPercentage {
				t.Fatalf("durability = %#v/%#v, want %#v/%#v", building["durability_status"], building["durability_percentage"], test.wantStatus, test.wantPercentage)
			}
			for _, key := range []string{"max_durability_seconds", "durability_remaining_seconds", "retention_remaining_seconds", "durability_expires_at", "status_expires_at"} {
				if _, ok := building[key]; ok {
					t.Fatalf("building exposes internal durability key %q: %#v", key, building)
				}
			}
		})
	}
}

func TestPlayerStateResponseLogsBuildingDurabilityComputation(t *testing.T) {
	fixture := newBuildingAPIFixture(t, "building-durability-computation-log")
	prepareCompletedBuilding(t, fixture, fixture.now.Add(24*time.Hour))
	requestID := "building-durability-computation-request"
	request := fixture.request(http.MethodGet, "/api/me", "", requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })

	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/me status = %d: %s", response.Code, response.Body.String())
	}
	want := "user_id=1 action=building_durability_calculation outcome=success building_id=1 durability_status=active remaining_seconds=86400 request_id=" + requestID
	if !strings.Contains(logOutput, want) {
		t.Fatalf("building durability computation log = %q, want %q", logOutput, want)
	}
	for _, secret := range []string{"session-token", "authorization-code", "request-body"} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("building durability log leaked %q: %q", secret, logOutput)
		}
	}
}

func TestRepairBuildingAPIRejectsInvalidRequestsWithAuthoritativeState(t *testing.T) {
	tests := []struct {
		name, body, reason string
	}{
		{"invalid JSON", `{`, repairReasonInvalidJSON},
		{"unknown field", `{"unexpected":1}`, repairReasonUnknownField},
		{"duplicate building", `{"building_id":1,"building_id":1}`, repairReasonDuplicate},
		{"extra JSON value", `{"building_id":1}{}`, repairReasonExtraValue},
		{"missing building", `{}`, repairReasonMissingBuilding},
		{"building string", `{"building_id":"1"}`, repairReasonInvalidBuilding},
		{"building zero", `{"building_id":0}`, repairReasonInvalidBuilding},
		{"building negative", `{"building_id":-1}`, repairReasonInvalidBuilding},
		{"unknown target", `{"building_id":999}`, repairReasonUnknownBuilding},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBuildingAPIFixture(t, "repair-invalid-"+strings.ReplaceAll(test.name, " ", "-"))
			prepareCompletedBuilding(t, fixture, fixture.now.Add(24*time.Hour))
			if _, err := fixture.store.db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 1)`, fixture.identity.ID); err != nil {
				t.Fatal(err)
			}
			before, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "repair-invalid-" + strings.ReplaceAll(test.name, " ", "-")
			request := fixture.request(http.MethodPost, "/api/actions/repair-building", test.body, requestID)
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusBadRequest || !strings.Contains(logOutput, "user_id=1 action=repair-building outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("rejection status/log = %d/%q", response.Code, logOutput)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] == nil || body["ap"] == nil || body["buildings"] == nil {
				t.Fatalf("failure lacks authoritative state = %#v", body)
			}
			after, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchangedBuildingState(t, before, after)
		})
	}
}

func TestRepairBuildingAPIRejectsDomainFailuresWithAuthoritativeState(t *testing.T) {
	tests := []struct {
		name, reason string
		prepare      func(*testing.T, buildingAPIFixture, int64)
	}{
		{"remote building", repairReasonRemote, func(t *testing.T, f buildingAPIFixture, _ int64) {
			if _, err := f.store.db.Exec(`INSERT INTO locations (id, display_name) VALUES ('remote-repair-location', 'Remote')`); err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.db.Exec(`UPDATE player_locations SET location_id = 'remote-repair-location' WHERE user_id = ?`, f.identity.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"under construction", repairReasonUnderConstruction, func(t *testing.T, f buildingAPIFixture, buildingID int64) {
			if _, err := f.store.db.Exec(`UPDATE buildings SET status = 'under_construction', durability_expires_at = NULL, contributed_ap = 0 WHERE id = ?`, buildingID); err != nil {
				t.Fatal(err)
			}
		}},
		{"insufficient AP", repairReasonInsufficientAP, func(t *testing.T, f buildingAPIFixture, _ int64) {
			if _, err := f.store.db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, f.now.Add(maxAP*time.Minute).Unix(), f.identity.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"insufficient resource", repairReasonInsufficientResource, func(_ *testing.T, _ buildingAPIFixture, _ int64) {
			return
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBuildingAPIFixture(t, "repair-domain-"+strings.ReplaceAll(test.name, " ", "-"))
			buildingID := prepareCompletedBuilding(t, fixture, fixture.now.Add(24*time.Hour))
			if test.name != "insufficient resource" {
				if _, err := fixture.store.db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 1)`, fixture.identity.ID); err != nil {
					t.Fatal(err)
				}
			}
			test.prepare(t, fixture, buildingID)
			before, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "repair-domain-" + strings.ReplaceAll(test.name, " ", "-")
			request := fixture.request(http.MethodPost, "/api/actions/repair-building", `{"building_id":`+strconv.FormatInt(buildingID, 10)+`}`, requestID)
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusConflict || !strings.Contains(logOutput, "user_id=1 action=repair-building outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("rejection status/log = %d/%q", response.Code, logOutput)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] == nil || body["ap"] == nil || body["buildings"] == nil {
				t.Fatalf("failure lacks authoritative state = %#v", body)
			}
			after, err := fixture.store.GetPlayerState(fixture.identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertUnchangedBuildingState(t, before, after)
		})
	}
}

func TestRepairBuildingAPIReturnsAuthoritativeStateAndSanitizedComputationLog(t *testing.T) {
	fixture := newBuildingAPIFixture(t, "repair-success")
	buildingID := prepareCompletedBuilding(t, fixture, fixture.now.Add(6*24*time.Hour))
	if _, err := fixture.store.db.Exec(`INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 2)`, fixture.identity.ID); err != nil {
		t.Fatal(err)
	}
	requestID := "repair-success-request"
	request := fixture.request(http.MethodPost, "/api/actions/repair-building", `{"building_id":`+strconv.FormatInt(buildingID, 10)+`}`, requestID)
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { fixture.server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("repair status = %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(logOutput, "user_id=1 action=repair-building outcome=success building_id=1 prior_durability_status=active added_seconds=3600 resulting_remaining_seconds=522000 ap_cost=10 wood_cost=1 request_id="+requestID) {
		t.Fatalf("repair computation log = %q", logOutput)
	}
	if !strings.Contains(logOutput, "user_id=1 action=repair-building outcome=success request_id="+requestID) {
		t.Fatalf("repair access log = %q", logOutput)
	}
	for _, secret := range []string{"session-token", "authorization-code", "request-body"} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("repair log leaked %q: %q", secret, logOutput)
		}
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP-10) {
		t.Fatalf("authoritative AP = %#v", body["ap"])
	}
	resources := responseResourceQuantities(t, body)
	if resources["wood"] != 1 {
		t.Fatalf("authoritative wood = %#v", resources["wood"])
	}
	building := buildingFromResponse(t, response.Body.Bytes())
	if building["durability_status"] != "active" || building["durability_percentage"] != float64(87) {
		t.Fatalf("authoritative durability = %#v", building)
	}
	for _, key := range []string{"max_durability_seconds", "durability_remaining_seconds", "retention_remaining_seconds", "durability_expires_at", "status_expires_at"} {
		if _, ok := building[key]; ok {
			t.Fatalf("building exposes internal durability key %q: %#v", key, building)
		}
	}
}

func TestCraftAPIUsesRecipeWhitelistAndReturnsAuthoritativeState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-craft", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)", identity.ID); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"}
	handler := server.Routes()

	request := httptest.NewRequest(http.MethodPost, "/api/actions/craft", strings.NewReader(`{"recipe_id":"wood_component"}`))
	request.AddCookie(cookie)
	request.Header.Set("X-Request-ID", "craft-request")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("craft status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP-10) {
		t.Fatalf("craft response state = %#v", body)
	}
	craftingRecipes, ok := body["crafting_recipes"].([]any)
	if !ok || len(craftingRecipes) != 0 {
		t.Fatalf("craft response recipes = %#v, want no recipes after consuming all inputs", body["crafting_recipes"])
	}
	inventory, ok := body["inventory"].([]any)
	if !ok || len(inventory) != 1 || inventory[0].(map[string]any)["quantity"] != float64(1) {
		t.Fatalf("craft response inventory = %#v", body["inventory"])
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-10 || playerResourceQuantity(state, "wood") != 0 || len(state.Inventory) != 1 || state.Inventory[0].Quantity != 1 {
		t.Fatalf("craft state = %#v", state)
	}

	beforeAP := state.AP
	for _, input := range []string{`{}`, `{"recipe_id":"unknown"}`, `{"recipe_id":"wood_component","extra":true}`, `{"recipe_id":"wood_component","recipe_id":"wood_component"}`, `[]`} {
		request := httptest.NewRequest(http.MethodPost, "/api/actions/craft", strings.NewReader(input))
		request.AddCookie(cookie)
		failure := httptest.NewRecorder()
		handler.ServeHTTP(failure, request)
		if failure.Code != http.StatusBadRequest {
			t.Fatalf("craft invalid input %s status = %d", input, failure.Code)
		}
	}
	state, err = store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != beforeAP || len(state.Inventory) != 1 || playerResourceQuantity(state, "wood") != 0 {
		t.Fatalf("invalid craft changed state = %#v", state)
	}
}

func TestCraftAPILogsSuccessWithoutSensitiveValues(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-craft-log-success", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 10)", identity.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/craft", strings.NewReader(`{"recipe_id":"wood_component"}`))
	request.Header.Set("X-Request-ID", "craft-log-success")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("craft status = %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(logOutput, "user_id=1 action=craft outcome=success request_id=craft-log-success") {
		t.Fatalf("craft success log = %q", logOutput)
	}
	for _, secret := range []string{"session-secret", "authorization-code-secret", "oauth-code-secret", "access-token", "refresh-token", "id-token", `{"recipe_id":"wood_component"}`} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("craft success log leaked %q: %q", secret, logOutput)
		}
	}
}

func TestCraftAPILogsRejectionReasonAndSanitizesInput(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-craft-log-rejection", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	rawInput := `{"recipe_id":"wood_component","raw_input":"oauth-code-secret"}`
	request := httptest.NewRequest(http.MethodPost, "/api/actions/craft", strings.NewReader(rawInput))
	request.Header.Set("X-Request-ID", "craft-log-rejection")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusBadRequest {
		t.Fatalf("craft rejection status = %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(logOutput, "user_id=1 action=craft outcome=error reason=unknown_field request_id=craft-log-rejection") {
		t.Fatalf("craft rejection log = %q", logOutput)
	}
	for _, secret := range []string{"session-secret", "authorization-code-secret", "oauth-code-secret", "access-token", "refresh-token", "id-token", rawInput} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("craft rejection log leaked %q: %q", secret, logOutput)
		}
	}
}

func TestGatherAPIUpdatesStateAndMeResponse(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-gather", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/gather", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://game.example.test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gather-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("gather status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP-30) || body["error"] != nil {
		t.Fatalf("gather response = %#v", body)
	}
	option, ok := body["gathering_option"].(map[string]any)
	if !ok || option["quantity"] != float64(1) || option["ap_cost"] != float64(10) {
		t.Fatalf("gathering option = %#v", body["gathering_option"])
	}
	item, ok := option["item"].(map[string]any)
	if !ok || item["id"] != "wood" || item["display_name"] != "Wood" {
		t.Fatalf("gathering item = %#v", option["item"])
	}
	inventory, ok := body["inventory"].([]any)
	if !ok || len(inventory) != 1 {
		t.Fatalf("gather inventory = %#v", body["inventory"])
	}
	entry, ok := inventory[0].(map[string]any)
	if !ok || entry["quantity"] != float64(1) {
		t.Fatalf("gather inventory entry = %#v", inventory[0])
	}
	if !strings.Contains(logOutput, "user_id=1 action=gather outcome=success request_id=gather-request") {
		t.Fatalf("gather log = %q", logOutput)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	meResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(meResponse, me)
	if meResponse.Code != http.StatusOK || !strings.Contains(meResponse.Body.String(), `"quantity":1`) {
		t.Fatalf("GET /api/me after gather = %d: %s", meResponse.Code, meResponse.Body.String())
	}
}

func TestGatherAPIRejectsInvalidPayloadAndPreservesState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-invalid-gather", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{name: "malformed", body: `{`, reason: gatherReasonInvalidJSON},
		{name: "unknown field", body: `{"item":"wood"}`, reason: gatherReasonUnknownField},
		{name: "duplicate field", body: `{"item":"wood","item":"wood"}`, reason: gatherReasonDuplicate},
		{name: "trailing value", body: `{}{}`, reason: gatherReasonExtraValue},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/actions/gather", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Request-ID", "invalid-gather-"+strings.ReplaceAll(test.name, " ", "-"))
			request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid gather status = %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(logOutput, "user_id=1 action=gather outcome=error reason="+test.reason) || !strings.Contains(logOutput, "request_id=invalid-gather-") {
				t.Fatalf("invalid gather log = %q", logOutput)
			}
			if strings.Contains(logOutput, test.body) {
				t.Fatalf("invalid gather log leaked request body: %q", logOutput)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["ap"] != float64(maxAP) {
				t.Fatalf("invalid gather response state = %#v", body)
			}
			state, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.Location.ID != "camp" || state.AP != maxAP || len(state.Inventory) != 0 {
				t.Fatalf("invalid gather changed state = %+v", state)
			}
		})
	}
}

func TestGatherAPIRejectsLocationAndInsufficientAPWithState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-gather-errors", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	locationRequest := httptest.NewRequest(http.MethodPost, "/api/actions/gather", strings.NewReader(`{}`))
	locationRequest.Header.Set("X-Request-ID", "gather-location-request")
	locationRequest.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	locationResponse := httptest.NewRecorder()
	locationLog := captureStdout(t, func() { server.Routes().ServeHTTP(locationResponse, locationRequest) })
	if locationResponse.Code != http.StatusBadRequest || !strings.Contains(locationResponse.Body.String(), `"error":"gathering not found"`) {
		t.Fatalf("location gather response = %d: %s", locationResponse.Code, locationResponse.Body.String())
	}
	if !strings.Contains(locationLog, "action=gather outcome=error reason="+gatherReasonInvalidLocation+" request_id=gather-location-request") {
		t.Fatalf("location gather log = %q", locationLog)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	lowAPRequest := httptest.NewRequest(http.MethodPost, "/api/actions/gather", strings.NewReader(`{}`))
	lowAPRequest.Header.Set("X-Request-ID", "gather-low-ap-request")
	lowAPRequest.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	lowAPResponse := httptest.NewRecorder()
	lowAPLog := captureStdout(t, func() { server.Routes().ServeHTTP(lowAPResponse, lowAPRequest) })
	if lowAPResponse.Code != http.StatusConflict || !strings.Contains(lowAPResponse.Body.String(), `"error":"insufficient action points"`) {
		t.Fatalf("low AP gather response = %d: %s", lowAPResponse.Code, lowAPResponse.Body.String())
	}
	if !strings.Contains(lowAPLog, "user_id=1 action=gather outcome=insufficient_ap request_id=gather-low-ap-request") {
		t.Fatalf("low AP gather log = %q", lowAPLog)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "forest_edge" || state.AP != 0 || len(state.Inventory) != 0 {
		t.Fatalf("failed gathers changed state = %+v", state)
	}
}

func TestConvertAPIUpdatesStateAndUsesBackendOwnedValues(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	store.essenceRoll = func() int { return 10000 }
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-convert", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)", identity.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{"method_id":"hand_wood_t1","quantity":1}`))
	request.Header.Set("Origin", "https://game.example.test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "convert-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("convert status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP-30) || body["error"] != nil {
		t.Fatalf("convert response = %#v", body)
	}
	resources := responseResourceQuantities(t, body)
	if len(resources) != 8 || resources["wood"] != float64(1) {
		t.Fatalf("convert resources = %#v", resources)
	}
	inventory, ok := body["inventory"].([]any)
	if !ok || len(inventory) != 0 {
		t.Fatalf("convert inventory = %#v", body["inventory"])
	}
	if !strings.Contains(logOutput, "user_id=1 action=convert outcome=success request_id=convert-request") {
		t.Fatalf("convert log = %q", logOutput)
	}
	if strings.Contains(logOutput, `"resource_yield"`) || strings.Contains(logOutput, "session-secret") {
		t.Fatalf("convert log leaked input or credentials: %q", logOutput)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	meResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(meResponse, me)
	if meResponse.Code != http.StatusOK || !strings.Contains(meResponse.Body.String(), `"resources"`) || !strings.Contains(meResponse.Body.String(), `"id":"wood","display_name":"Wood"`) {
		t.Fatalf("GET /api/me after convert = %d: %s", meResponse.Code, meResponse.Body.String())
	}
	var persistedBody map[string]any
	if err := json.Unmarshal([]byte(meResponse.Body.String()), &persistedBody); err != nil {
		t.Fatal(err)
	}
	persistedResources := responseResourceQuantities(t, persistedBody)
	if persistedResources["wood"] != float64(1) {
		t.Fatalf("GET /api/me persisted Wood quantity = %v, want 1", persistedResources["wood"])
	}
}

func TestConvertAPIRejectsProviderForGlobalMethodWithoutChangingState(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-global-provider", "global-provider@example.com", "Global Provider")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO conversion_methods (id, display_name, ap_cost, input_item_id, max_input_quantity, output_resource_id, resource_quantity_per_input, essence_item_id, essence_chance_bps, essence_quantity) VALUES ('global_wood', 'Global Wood', 5, 'wood', 1, 'wood', 2, NULL, 0, 0); INSERT INTO global_conversion_methods (conversion_method_id) VALUES ('global_wood'); INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{"method_id":"global_wood","quantity":1,"provider_extension_id":0}`))
	request.Header.Set("X-Request-ID", "global-provider-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error":"invalid provider extension"`) {
		t.Fatalf("global provider response = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP) || responseResourceQuantities(t, body)["wood"] != 0 {
		t.Fatalf("global provider response state = %#v", body)
	}
	if !strings.Contains(logOutput, "user_id=1 action=convert outcome=error reason=invalid_provider request_id=global-provider-request") {
		t.Fatalf("global provider log = %q", logOutput)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP || playerResourceQuantity(state, "wood") != 0 || inventoryQuantity(state, "wood") != 1 {
		t.Fatalf("global provider changed state = %+v", state)
	}
}

func TestLegacyConvertAPIUsesEmptyObjectContract(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-api-legacy-convert", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)", identity.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{}`))
	request.Header.Set("X-Request-ID", "legacy-convert-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("legacy convert status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ap"] != float64(maxAP-1) || body["error"] != nil || body["method_id"] != "legacy" || body["quantity"] != float64(1) || body["resource_quantity"] != float64(1) || body["essence_quantity"] != float64(0) {
		t.Fatalf("legacy convert response = %#v", body)
	}
	if resources := responseResourceQuantities(t, body); len(resources) != 8 || resources["wood"] != 1 {
		t.Fatalf("legacy convert resources = %#v", resources)
	}
	if inventory, ok := body["inventory"].([]any); !ok || len(inventory) != 0 {
		t.Fatalf("legacy convert inventory = %#v", body["inventory"])
	}
	if !strings.Contains(logOutput, "user_id=1 action=convert method_id=legacy quantity=1 resource_quantity=1 essence_quantity=0 essence_result=reported outcome=success request_id=legacy-convert-request") {
		t.Fatalf("legacy convert log = %q", logOutput)
	}
	if strings.Contains(logOutput, "session-secret") || strings.Contains(logOutput, "{}") {
		t.Fatalf("legacy convert log leaked credentials or raw input: %q", logOutput)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != maxAP-1 || playerResourceQuantity(state, "wood") != 1 || len(state.Inventory) != 0 {
		t.Fatalf("legacy convert persisted state = %+v", state)
	}
}

func TestConvertAPIRejectsInvalidPayloadAndPreservesState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-invalid-convert", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{name: "malformed", body: `{`, reason: convertReasonInvalidJSON},
		{name: "empty field", body: `{"":1}`, reason: convertReasonUnknownField},
		{name: "unknown field", body: `{"resource_yield":99}`, reason: convertReasonUnknownField},
		{name: "duplicate field", body: `{"method_id":"hand_wood_t1","method_id":"hand_wood_t1","quantity":1}`, reason: convertReasonDuplicate},
		{name: "trailing value", body: `{"method_id":"hand_wood_t1","quantity":1}{}`, reason: convertReasonExtraValue},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Request-ID", "invalid-convert-"+strings.ReplaceAll(test.name, " ", "-"))
			request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid convert status = %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(logOutput, "user_id=1 action=convert outcome=error reason="+test.reason) || !strings.Contains(logOutput, "request_id=invalid-convert-") {
				t.Fatalf("invalid convert log = %q", logOutput)
			}
			if strings.Contains(logOutput, test.body) || strings.Contains(logOutput, "session-secret") {
				t.Fatalf("invalid convert log leaked input or credentials: %q", logOutput)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["ap"] != float64(maxAP) {
				t.Fatalf("invalid convert response state = %#v", body)
			}
			resources := responseResourceQuantities(t, body)
			if len(resources) != 8 || resources["wood"] != 0 {
				t.Fatalf("invalid convert response resources = %#v", resources)
			}
			state, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.Location.ID != "camp" || state.AP != maxAP || playerResourceQuantity(state, "wood") != 0 || len(state.Inventory) != 0 {
				t.Fatalf("invalid convert changed state = %+v", state)
			}
		})
	}
}

func TestConvertAPIRejectsLocationAndMissingWoodWithState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-errors", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Move(identity.ID, "forest_edge"); err != nil {
		t.Fatal(err)
	}
	locationRequest := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{"method_id":"hand_wood_t1","quantity":1}`))
	locationRequest.Header.Set("X-Request-ID", "convert-location-request")
	locationRequest.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	locationResponse := httptest.NewRecorder()
	locationLog := captureStdout(t, func() { server.Routes().ServeHTTP(locationResponse, locationRequest) })
	if locationResponse.Code != http.StatusConflict || !strings.Contains(locationResponse.Body.String(), `"error":"insufficient item"`) {
		t.Fatalf("location convert response = %d: %s", locationResponse.Code, locationResponse.Body.String())
	}
	var locationBody map[string]any
	if err := json.Unmarshal(locationResponse.Body.Bytes(), &locationBody); err != nil {
		t.Fatal(err)
	}
	if resources := responseResourceQuantities(t, locationBody); len(resources) != 8 || resources["wood"] != 0 {
		t.Fatalf("location convert resources = %#v", resources)
	}
	if !strings.Contains(locationLog, "action=convert outcome=error reason="+convertReasonInsufficientItem+" request_id=convert-location-request") {
		t.Fatalf("location convert log = %q", locationLog)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "forest_edge" || state.AP != maxAP-20 || playerResourceQuantity(state, "wood") != 0 {
		t.Fatalf("location convert changed state = %+v", state)
	}

	if _, err := store.Move(identity.ID, "camp"); err != nil {
		t.Fatal(err)
	}
	itemRequest := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{"method_id":"hand_wood_t1","quantity":1}`))
	itemRequest.Header.Set("X-Request-ID", "convert-item-request")
	itemRequest.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	itemResponse := httptest.NewRecorder()
	itemLog := captureStdout(t, func() { server.Routes().ServeHTTP(itemResponse, itemRequest) })
	if itemResponse.Code != http.StatusConflict || !strings.Contains(itemResponse.Body.String(), `"error":"insufficient item"`) {
		t.Fatalf("missing Wood convert response = %d: %s", itemResponse.Code, itemResponse.Body.String())
	}
	var itemBody map[string]any
	if err := json.Unmarshal(itemResponse.Body.Bytes(), &itemBody); err != nil {
		t.Fatal(err)
	}
	if resources := responseResourceQuantities(t, itemBody); len(resources) != 8 || resources["wood"] != 0 {
		t.Fatalf("missing Wood convert resources = %#v", resources)
	}
	if !strings.Contains(itemLog, "action=convert outcome=error reason="+convertReasonInsufficientItem+" request_id=convert-item-request") {
		t.Fatalf("missing Wood convert log = %q", itemLog)
	}
	state, err = store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.AP != maxAP-40 || playerResourceQuantity(state, "wood") != 0 || len(state.Inventory) != 0 {
		t.Fatalf("missing Wood convert changed state = %+v", state)
	}
}

func TestConvertAPIRejectsInsufficientAPWithoutChangingState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-convert-low-ap", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)", identity.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{"method_id":"hand_wood_t1","quantity":1}`))
	request.Header.Set("X-Request-ID", "convert-low-ap-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"error":"insufficient action points"`) {
		t.Fatalf("low AP convert response = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if resources := responseResourceQuantities(t, body); len(resources) != 8 || resources["wood"] != 0 {
		t.Fatalf("low AP convert resources = %#v", resources)
	}
	if !strings.Contains(logOutput, "action=convert outcome=error reason="+convertReasonInsufficientAP+" request_id=convert-low-ap-request") {
		t.Fatalf("low AP convert log = %q", logOutput)
	}
	if strings.Contains(logOutput, "action=convert outcome=insufficient_ap") {
		t.Fatalf("low AP convert log used a non-rejection outcome: %q", logOutput)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.AP != 0 || playerResourceQuantity(state, "wood") != 0 || len(state.Inventory) != 1 || state.Inventory[0].Quantity != 1 {
		t.Fatalf("low AP convert changed state = %+v", state)
	}
}

func TestMoveAPIUpdatesLocationAndAP(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-move", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/move", strings.NewReader(`{"target":"forest_edge"}`))
	request.Header.Set("Origin", "https://game.example.test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "move-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	location, ok := body["location"].(map[string]any)
	if !ok || location["id"] != "forest_edge" || body["ap"] != float64(maxAP-20) || body["carried_weight"] != float64(0) || body["movement_weight_threshold"] != float64(movementWeightThreshold) {
		t.Fatalf("move response = %#v", body)
	}
	if !strings.Contains(logOutput, "user_id="+strconv.FormatInt(identity.ID, 10)+" action=carrying_weight_calculation outcome=success carried_weight=0 movement_weight_threshold=1000 request_id=move-request") {
		t.Fatalf("move carrying weight computation log = %q", logOutput)
	}
	if _, hasError := body["error"]; hasError {
		t.Fatalf("successful move returned error: %#v", body)
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "forest_edge" || state.AP != maxAP-20 || len(state.Routes) != 1 || state.Routes[0].DestinationID != "camp" {
		t.Fatalf("stored move state = %+v", state)
	}
}

func TestMoveAPIRejectsInvalidInputAndLogsSafeReason(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-invalid-move", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{name: "unknown field", body: `{"target":"forest_edge","cost":20}`, reason: moveReasonUnknownField},
		{name: "extra value", body: `{"target":"forest_edge"}{}`, reason: moveReasonExtraValue},
		{name: "invalid target type", body: `{"target":20}`, reason: moveReasonInvalidTarget},
		{name: "missing target", body: `{}`, reason: moveReasonMissingTarget},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/actions/move", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Request-ID", "invalid-"+strings.ReplaceAll(test.name, " ", "-"))
			request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid move status = %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(logOutput, "user_id=1 action=move outcome=error reason="+test.reason) || !strings.Contains(logOutput, "request_id=invalid-") {
				t.Fatalf("invalid move log = %q", logOutput)
			}
			if strings.Contains(logOutput, test.body) {
				t.Fatalf("invalid move log leaked request body: %q", logOutput)
			}
			state, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.Location.ID != "camp" || state.AP != maxAP {
				t.Fatalf("invalid move changed state = %+v", state)
			}
		})
	}
}

func TestMoveAPIInsufficientAPPreservesState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-low-ap", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/move", strings.NewReader(`{"target":"forest_edge"}`))
	request.Header.Set("X-Request-ID", "low-ap-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("insufficient move status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != ErrInsufficientAP.Error() || body["ap"] != float64(0) {
		t.Fatalf("insufficient move response = %#v", body)
	}
	location, ok := body["location"].(map[string]any)
	if !ok || location["id"] != "camp" {
		t.Fatalf("insufficient move location = %#v", body["location"])
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.AP != 0 {
		t.Fatalf("insufficient move changed state = %+v", state)
	}
}

func TestMoveAPIRejectsOverweightWithAuthoritativeStateAndSafeComputationLog(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-overweight-move-api", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 11)", identity.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	requestID := "overweight-move-request"
	request := httptest.NewRequest(http.MethodPost, "/api/actions/move", strings.NewReader(`{"target":"forest_edge"}`))
	request.Header.Set("X-Request-ID", requestID)
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })

	if response.Code != http.StatusConflict {
		t.Fatalf("overweight move status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	location, ok := body["location"].(map[string]any)
	if !ok || body["error"] != ErrOverweight.Error() || location["id"] != "camp" || body["ap"] != float64(maxAP) || body["carried_weight"] != float64(1100) || body["movement_weight_threshold"] != float64(movementWeightThreshold) {
		t.Fatalf("overweight move response = %#v", body)
	}
	wantWeightLog := "user_id=" + strconv.FormatInt(identity.ID, 10) + " action=carrying_weight_calculation outcome=success carried_weight=1100 movement_weight_threshold=1000 request_id=" + requestID
	if !strings.Contains(logOutput, wantWeightLog) {
		t.Fatalf("overweight carrying weight computation log = %q, want %q", logOutput, wantWeightLog)
	}
	wantRejectionLog := "user_id=" + strconv.FormatInt(identity.ID, 10) + " action=move outcome=error reason=overweight carried_weight=1100 movement_weight_threshold=1000 request_id=" + requestID
	if !strings.Contains(logOutput, wantRejectionLog) {
		t.Fatalf("overweight rejection log = %q, want %q", logOutput, wantRejectionLog)
	}
	for _, secret := range []string{"session-secret", `{"target":"forest_edge"}`} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("overweight move log leaked %q: %q", secret, logOutput)
		}
	}
	state, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Location.ID != "camp" || state.AP != maxAP || state.CarriedWeight != 1100 {
		t.Fatalf("overweight move changed authoritative state = %+v", state)
	}
}

func TestUnknownActionIsRejectedAndLogged(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-unknown-action", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	const unknownActionIdentifier = "attack-secret"
	request := httptest.NewRequest(http.MethodPost, "/api/actions/"+unknownActionIdentifier, nil)
	request.Header.Set("X-Request-ID", "unknown-action-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown action status = %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(logOutput, "user_id=1 action=unknown outcome=error reason="+moveReasonUnsupported+" request_id=unknown-action-request") {
		t.Fatalf("unknown action log = %q", logOutput)
	}
	if strings.Contains(logOutput, unknownActionIdentifier) {
		t.Fatalf("unknown action identifier leaked into log: %q", logOutput)
	}
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	run()
	_ = writer.Close()
	os.Stdout = oldStdout
	logOutput, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(logOutput)
}

func TestRestInsufficientAPReturnsConflictWithoutChangingState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-1", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := store.db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/rest", nil)
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("insufficient AP status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != ErrInsufficientAP.Error() || body["ap"] != float64(0) {
		t.Fatalf("insufficient AP JSON = %#v", body)
	}
	availableActions, ok := body["available_actions"].([]any)
	if !ok || len(availableActions) != 0 {
		t.Fatalf("insufficient AP available actions = %#v, want none", body["available_actions"])
	}
	var after int64
	if err := store.db.QueryRow("SELECT full_timestamp FROM player_ap WHERE user_id = ?", identity.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("insufficient rest changed full timestamp from %d to %d", before, after)
	}
}

func TestRestRejectsForeignOriginBeforeChangingState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-foreign-origin", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/actions/rest", nil)
	request.Header.Set("Origin", "https://evil.example.test")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["error"] != "origin not allowed" {
		t.Fatalf("foreign origin JSON = %#v", body)
	}
	if ap, err := store.GetAP(identity.ID); err != nil || ap != maxAP {
		t.Fatalf("foreign origin changed AP: ap=%d err=%v", ap, err)
	}
}

func TestRestCORSPreflightAllowsTrustedPostOnly(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, _ := newTestServer(t, &fakeProvider{}, &now)
	handler := server.Routes()

	trusted := httptest.NewRequest(http.MethodOptions, "/api/actions/rest", nil)
	trusted.Header.Set("Origin", "https://game.example.test")
	trusted.Header.Set("Access-Control-Request-Method", http.MethodPost)
	trustedResponse := httptest.NewRecorder()
	handler.ServeHTTP(trustedResponse, trusted)
	if trustedResponse.Code != http.StatusNoContent || trustedResponse.Header().Get("Access-Control-Allow-Origin") != "https://game.example.test" || !strings.Contains(trustedResponse.Header().Get("Access-Control-Allow-Methods"), http.MethodPost) {
		t.Fatalf("trusted POST preflight = %d, origin=%q, methods=%q", trustedResponse.Code, trustedResponse.Header().Get("Access-Control-Allow-Origin"), trustedResponse.Header().Get("Access-Control-Allow-Methods"))
	}

	foreign := httptest.NewRequest(http.MethodOptions, "/api/actions/rest", nil)
	foreign.Header.Set("Origin", "https://evil.example.test")
	foreign.Header.Set("Access-Control-Request-Method", http.MethodPost)
	foreignResponse := httptest.NewRecorder()
	handler.ServeHTTP(foreignResponse, foreign)
	if foreignResponse.Header().Get("Access-Control-Allow-Origin") != "" || strings.Contains(foreignResponse.Header().Get("Access-Control-Allow-Origin"), "*") {
		t.Fatal("foreign POST preflight received CORS authorization")
	}
}

func TestAccessLogIncludesRequestIDAndOmitsSessionSecret(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-1", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	request := httptest.NewRequest(http.MethodPost, "/api/actions/rest", nil)
	request.Header.Set("X-Request-ID", "request-123")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	_ = writer.Close()
	os.Stdout = oldStdout
	logOutput, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	text := string(logOutput)
	if !strings.Contains(text, "user_id=1") || !strings.Contains(text, "request_id=request-123") || !strings.Contains(text, "action=rest") || !strings.Contains(text, "action=ap_calculation") {
		t.Fatalf("access log lacks required fields: %q", text)
	}
	if strings.Contains(text, "session-secret") {
		t.Fatalf("access log leaked session secret: %q", text)
	}
}

func TestGroundTransferAPIReturnsTypedStateAndKeepsTransferOutOfActions(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-api", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 5);
INSERT INTO player_resources (user_id, resource_id, quantity) VALUES (?, 'wood', 7)`, identity.ID, identity.ID); err != nil {
		t.Fatal(err)
	}
	handler := server.Routes()
	request := httptest.NewRequest(http.MethodPost, "/api/transfers/drop", strings.NewReader(`{"asset_type":"item","asset_id":"wood","quantity":2,"item_status":"active"}`))
	request.Header.Set("X-Request-ID", "ground-drop-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { handler.ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("drop status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, exists := body["error"]; exists {
		t.Fatalf("successful transfer returned error: %#v", body)
	}
	if body["ap"] != float64(maxAP) {
		t.Fatalf("drop changed AP: %#v", body["ap"])
	}
	groundItems, ok := body["ground_items"].([]any)
	if !ok || len(groundItems) != 1 {
		t.Fatalf("ground item response = %#v", body["ground_items"])
	}
	groundItem := groundItems[0].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(groundItem), []string{"durability_percentage", "durability_status", "item", "quantity"}) || groundItem["quantity"] != float64(2) || groundItem["durability_status"] != "active" || groundItem["durability_percentage"] != float64(100) {
		t.Fatalf("ground item shape = %#v", groundItem)
	}
	for _, key := range []string{"max_durability_seconds", "durability_remaining_seconds", "retention_remaining_seconds", "durability_expires_at", "status_expires_at"} {
		if _, ok := groundItem[key]; ok {
			t.Fatalf("ground item exposes internal durability key %q: %#v", key, groundItem)
		}
	}
	item := groundItem["item"].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(item), []string{"display_name", "id"}) || item["id"] != "wood" || item["display_name"] != "Wood" {
		t.Fatalf("ground item identity = %#v", item)
	}
	if !strings.Contains(logOutput, "user_id=1 action=transfer-drop location_id=camp asset_type=item asset_id=wood quantity=2 outcome=success reason=none request_id=ground-drop-request") {
		t.Fatalf("drop computation log = %q", logOutput)
	}
	if !strings.Contains(logOutput, "user_id=1 action=transfer-drop outcome=success request_id=ground-drop-request") {
		t.Fatalf("drop access log = %q", logOutput)
	}
	if strings.Contains(logOutput, "api/actions") {
		t.Fatalf("transfer was routed as an Action: %q", logOutput)
	}

	resourceDrop := httptest.NewRequest(http.MethodPost, "/api/transfers/drop", strings.NewReader(`{"asset_type":"resource","asset_id":"wood","quantity":2}`))
	resourceDrop.Header.Set("X-Request-ID", "ground-resource-drop-request")
	resourceDrop.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	resourceDropResponse := httptest.NewRecorder()
	resourceDropLog := captureStdout(t, func() { handler.ServeHTTP(resourceDropResponse, resourceDrop) })
	if resourceDropResponse.Code != http.StatusOK || !strings.Contains(resourceDropLog, "asset_type=resource asset_id=wood quantity=2 outcome=success") {
		t.Fatalf("resource drop status/log = %d/%q", resourceDropResponse.Code, resourceDropLog)
	}
	pickup := httptest.NewRequest(http.MethodPost, "/api/transfers/pickup", strings.NewReader(`{"asset_type":"resource","asset_id":"wood","quantity":1}`))
	pickup.Header.Set("X-Request-ID", "ground-pickup-request")
	pickup.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	pickupResponse := httptest.NewRecorder()
	pickupLog := captureStdout(t, func() { handler.ServeHTTP(pickupResponse, pickup) })
	if pickupResponse.Code != http.StatusOK {
		t.Fatalf("pickup status = %d: %s", pickupResponse.Code, pickupResponse.Body.String())
	}
	var pickupBody map[string]any
	if err := json.Unmarshal(pickupResponse.Body.Bytes(), &pickupBody); err != nil {
		t.Fatal(err)
	}
	groundResources, ok := pickupBody["ground_resources"].([]any)
	if !ok || len(groundResources) != 1 {
		t.Fatalf("ground resource response = %#v", pickupBody["ground_resources"])
	}
	groundResource := groundResources[0].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(groundResource), []string{"quantity", "resource"}) || groundResource["quantity"] != float64(1) {
		t.Fatalf("ground resource shape = %#v", groundResource)
	}
	resource := groundResource["resource"].(map[string]any)
	if !reflect.DeepEqual(sortedMapKeys(resource), []string{"display_name", "id"}) || resource["id"] != "wood" || resource["display_name"] != "Wood" {
		t.Fatalf("ground resource identity = %#v", resource)
	}
	if !strings.Contains(pickupLog, "user_id=1 action=transfer-pickup location_id=camp asset_type=resource asset_id=wood quantity=1 outcome=success reason=none request_id=ground-pickup-request") {
		t.Fatalf("pickup computation log = %q", pickupLog)
	}
}

func TestGroundTransferAPIRejectsStrictInputAndReturnsAuthoritativeState(t *testing.T) {
	tests := []struct {
		name, body, reason string
	}{
		{"invalid JSON", `{`, transferReasonInvalidJSON},
		{"unknown field", `{"asset_type":"item","asset_id":"wood","quantity":1,"secret":"credential-sentinel"}`, transferReasonUnknownField},
		{"duplicate field", `{"asset_type":"item","asset_type":"item","asset_id":"wood","quantity":1}`, transferReasonDuplicate},
		{"extra JSON value", `{"asset_type":"item","asset_id":"wood","quantity":1}{}`, transferReasonExtraValue},
		{"missing asset type", `{"asset_id":"wood","quantity":1}`, transferReasonMissingAssetType},
		{"invalid asset type", `{"asset_type":"currency","asset_id":"wood","quantity":1}`, transferReasonInvalidAssetType},
		{"missing asset ID", `{"asset_type":"item","quantity":1}`, transferReasonMissingAssetID},
		{"invalid asset ID", `{"asset_type":"item","asset_id":" ","quantity":1}`, transferReasonInvalidAssetID},
		{"missing quantity", `{"asset_type":"item","asset_id":"wood"}`, transferReasonMissingQuantity},
		{"invalid quantity", `{"asset_type":"item","asset_id":"wood","quantity":0}`, transferReasonInvalidQuantity},
		{"fraction quantity", `{"asset_type":"item","asset_id":"wood","quantity":1.5}`, transferReasonInvalidQuantity},
		{"missing item status", `{"asset_type":"item","asset_id":"wood","quantity":1}`, transferReasonMissingItemStatus},
		{"invalid item status", `{"asset_type":"item","asset_id":"wood","quantity":1,"item_status":"stale"}`, transferReasonInvalidItemStatus},
		{"resource item status", `{"asset_type":"resource","asset_id":"wood","quantity":1,"item_status":"active"}`, transferReasonResourceItemStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
			server, store := newTestServer(t, &fakeProvider{}, &now)
			identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-invalid-"+strings.ReplaceAll(test.name, " ", "-"), "person@example.com", "Person")
			if err != nil {
				t.Fatal(err)
			}
			if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 2)`, identity.ID); err != nil {
				t.Fatal(err)
			}
			before, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			requestID := "ground-invalid-" + strings.ReplaceAll(test.name, " ", "-")
			request := httptest.NewRequest(http.MethodPost, "/api/transfers/drop", strings.NewReader(test.body))
			request.Header.Set("X-Request-ID", requestID)
			request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
			if response.Code != http.StatusBadRequest || !strings.Contains(logOutput, "user_id=1 action=transfer-drop location_id=camp asset_type=unknown asset_id=unknown quantity=0 outcome=error reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("invalid transfer status/log = %d/%q", response.Code, logOutput)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] == nil || body["ap"] == nil || body["ground_items"] == nil || body["ground_resources"] == nil {
				t.Fatalf("invalid transfer lacks authoritative state = %#v", body)
			}
			if strings.Contains(logOutput, "credential-sentinel") {
				t.Fatalf("invalid transfer log leaked raw input: %q", logOutput)
			}
			after, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("invalid transfer changed state: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestGroundTransferAPIDomainFailuresAndAuthenticationContract(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-ground-domain", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood', 1)`, identity.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetPlayerState(identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, body, reason string
		status             int
	}{
		{"unknown asset", `{"asset_type":"item","asset_id":"unknown","quantity":1,"item_status":"active"}`, "unknown_asset", http.StatusBadRequest},
		{"insufficient source", `{"asset_type":"item","asset_id":"wood","quantity":2,"item_status":"active"}`, "insufficient_source", http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			requestID := "ground-domain-" + strings.ReplaceAll(test.name, " ", "-")
			request := httptest.NewRequest(http.MethodPost, "/api/transfers/drop", strings.NewReader(test.body))
			request.Header.Set("X-Request-ID", requestID)
			request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
			response := httptest.NewRecorder()
			logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
			if response.Code != test.status || !strings.Contains(logOutput, "user_id=1 action=transfer-drop location_id=camp asset_type=item") || !strings.Contains(logOutput, "reason="+test.reason+" request_id="+requestID) {
				t.Fatalf("domain failure status/log = %d/%q", response.Code, logOutput)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] == nil || body["ap"] == nil || body["ground_items"] == nil || body["ground_resources"] == nil {
				t.Fatalf("domain failure lacks authoritative state = %#v", body)
			}
			after, err := store.GetPlayerState(identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("domain failure changed state: before=%+v after=%+v", before, after)
			}
		})
	}

	unauthenticated := httptest.NewRequest(http.MethodPost, "/api/transfers/pickup", strings.NewReader(`{"asset_type":"item","asset_id":"wood","quantity":1}`))
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, unauthenticated)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated transfer status = %d", response.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["error"] != "authentication required" {
		t.Fatalf("unauthenticated transfer body = %#v", body)
	}
}

func TestItemDurabilityAPIExposesStatesAndRejectsExpiredPickup(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-api", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'wood', 'active', ?, 2), (?, 'wood', 'expired', ?, 3);
INSERT INTO ground_items (location_id, item_id, durability_status, status_expires_at, quantity) VALUES ('camp', 'wood', 'active', ?, 4), ('camp', 'wood', 'expired', ?, 5)`, identity.ID, now.Add(time.Hour).Unix(), identity.ID, now.Add(2*time.Hour).Unix(), now.Add(3*time.Hour).Unix(), now.Add(4*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.Header.Set("X-Request-ID", "item-state-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	logOutput := captureStdout(t, func() { server.Routes().ServeHTTP(response, request) })
	if response.Code != http.StatusOK {
		t.Fatalf("state status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	inventory := body["inventory"].([]any)
	if len(inventory) != 2 {
		t.Fatalf("inventory rows = %#v", inventory)
	}
	active := inventory[0].(map[string]any)
	expired := inventory[1].(map[string]any)
	if active["durability_status"] != "active" || active["durability_percentage"] != float64(100) {
		t.Fatalf("active durability = %#v", active)
	}
	if expired["durability_status"] != "expired" || expired["durability_percentage"] != float64(0) {
		t.Fatalf("expired durability = %#v", expired)
	}
	for _, entry := range []map[string]any{active, expired} {
		for _, key := range []string{"max_durability_seconds", "durability_remaining_seconds", "retention_remaining_seconds", "durability_expires_at", "status_expires_at"} {
			if _, ok := entry[key]; ok {
				t.Fatalf("inventory exposes internal durability key %q: %#v", key, entry)
			}
		}
	}
	if !strings.Contains(logOutput, "action=item_durability_calculation outcome=success") || !strings.Contains(logOutput, "durability_status=expired") || !strings.Contains(logOutput, "request_id=item-state-request") {
		t.Fatalf("durability log = %q", logOutput)
	}

	pickup := httptest.NewRequest(http.MethodPost, "/api/transfers/pickup", strings.NewReader(`{"asset_type":"item","asset_id":"wood","quantity":1,"item_status":"expired"}`))
	pickup.Header.Set("X-Request-ID", "expired-pickup-request")
	pickup.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	pickupResponse := httptest.NewRecorder()
	pickupLog := captureStdout(t, func() { server.Routes().ServeHTTP(pickupResponse, pickup) })
	if pickupResponse.Code != http.StatusConflict {
		t.Fatalf("expired pickup status = %d: %s", pickupResponse.Code, pickupResponse.Body.String())
	}
	var pickupBody map[string]any
	if err := json.Unmarshal(pickupResponse.Body.Bytes(), &pickupBody); err != nil {
		t.Fatal(err)
	}
	if pickupBody["inventory"] == nil || pickupBody["ground_items"] == nil || !strings.Contains(pickupLog, "reason=expired_item") || !strings.Contains(pickupLog, "item_status=expired") {
		t.Fatalf("expired pickup state/log = %#v/%q", pickupBody, pickupLog)
	}
	if _, err := store.db.Exec(`UPDATE player_inventory SET status_expires_at = ? WHERE user_id = ? AND item_id = 'wood' AND durability_status = 'expired'`, now.Add(-itemExpiredRetention-time.Second).Unix(), identity.ID); err != nil {
		t.Fatal(err)
	}
	cleanupRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	cleanupRequest.Header.Set("X-Request-ID", "item-cleanup-request")
	cleanupRequest.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	cleanupResponse := httptest.NewRecorder()
	cleanupLog := captureStdout(t, func() { server.Routes().ServeHTTP(cleanupResponse, cleanupRequest) })
	if cleanupResponse.Code != http.StatusOK || !strings.Contains(cleanupLog, "action=item_durability_cleanup outcome=success") || !strings.Contains(cleanupLog, "cleanup_action=deleted") || !strings.Contains(cleanupLog, "request_id=item-cleanup-request") {
		t.Fatalf("cleanup status/log = %d/%q", cleanupResponse.Code, cleanupLog)
	}
}

func TestDurabilityPercentageAPIUsesCeilingCapAndZero(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	server, store := newTestServer(t, &fakeProvider{}, &now)
	identity, err := store.UpsertIdentity("https://accounts.google.com", "subject-item-percentage", "person@example.com", "Person")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(identity.ID, "session-secret", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
INSERT INTO items (id, display_name, weight_units, max_durability_seconds) VALUES
('percentage_ceil', 'Percentage Ceil', 1, 3),
('percentage_cap', 'Percentage Cap', 1, 100),
('percentage_expired', 'Percentage Expired', 1, 100)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'percentage_ceil', 'active', ?, 1)`, identity.ID, now.Add(2*time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'percentage_cap', 'active', ?, 1)`, identity.ID, now.Add(101*time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, durability_status, status_expires_at, quantity) VALUES (?, 'percentage_expired', 'expired', ?, 1)`, identity.ID, now.Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO ground_items (location_id, item_id, durability_status, status_expires_at, quantity) VALUES ('camp', 'percentage_ceil', 'active', ?, 1)`, now.Add(time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.Header.Set("X-Request-ID", "item-percentage-request")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("state status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"max_durability_seconds", "durability_remaining_seconds", "retention_remaining_seconds", "durability_expires_at", "status_expires_at"} {
		if _, ok := body[key]; ok {
			t.Fatalf("state exposes internal durability key %q: %#v", key, body)
		}
	}
	inventory := body["inventory"].([]any)
	wantInventory := map[string]float64{"percentage_ceil": 67, "percentage_cap": 100, "percentage_expired": 0}
	for _, raw := range inventory {
		entry := raw.(map[string]any)
		item := entry["item"].(map[string]any)
		itemID := item["id"].(string)
		if entry["durability_percentage"] != wantInventory[itemID] {
			t.Fatalf("inventory %s percentage = %#v, want %v", itemID, entry["durability_percentage"], wantInventory[itemID])
		}
		if _, ok := entry["durability_remaining_seconds"]; ok {
			t.Fatalf("inventory %s exposes remaining seconds: %#v", itemID, entry)
		}
		if _, ok := entry["retention_remaining_seconds"]; ok {
			t.Fatalf("inventory %s exposes retention seconds: %#v", itemID, entry)
		}
	}
	groundItems := body["ground_items"].([]any)
	ground := groundItems[0].(map[string]any)
	if ground["durability_status"] != "active" || ground["durability_percentage"] != float64(34) {
		t.Fatalf("ground percentage = %#v", ground)
	}
}
