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
	if len(meBody) != 13 || meBody["id"] != float64(identity.ID) || meBody["display_name"] != "Person" || meBody["email"] != "person@example.com" || meBody["ap"] != float64(maxAP) {
		t.Fatalf("GET /api/me JSON = %#v", meBody)
	}
	buildingRecipes, ok := meBody["building_recipes"].([]any)
	if !ok || len(buildingRecipes) != 1 {
		t.Fatalf("GET /api/me building recipes = %#v", meBody["building_recipes"])
	}
	buildings, ok := meBody["buildings"].([]any)
	if !ok || len(buildings) != 0 {
		t.Fatalf("GET /api/me buildings = %#v", meBody["buildings"])
	}
	recipes, ok := meBody["crafting_recipes"].([]any)
	if !ok || len(recipes) != 1 {
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
	conversion, ok := meBody["conversion_option"].(map[string]any)
	outputResource, outputResourceOK := conversion["resource"].(map[string]any)
	if !outputResourceOK || outputResource["id"] != "wood" || outputResource["display_name"] != "Wood" || conversion["input_quantity"] != float64(1) || conversion["resource_yield"] != float64(1) || conversion["ap_cost"] != float64(1) {
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
	if len(restBody) != 1 || restBody["ap"] != float64(maxAP-1) {
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
	if _, err := store.db.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, 'wood_component', 1)`, identity.ID); err != nil {
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
	if !reflect.DeepEqual(sortedMapKeys(building), []string{"building_level", "contributed_ap", "extension_slot_count", "id", "owner", "recipe", "required_ap", "status"}) || building["required_ap"] != float64(60) || building["contributed_ap"] != float64(0) || building["status"] != "under_construction" {
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
	if _, err := store.db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, now.Add(time.Duration(maxAP)*time.Minute).UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(`{"building_id":1,"ap":1}`))
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("insufficient AP contribution status = %d", response.Code)
	}
	if _, err := store.db.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?`, now.UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/contribute-construction", strings.NewReader(`{"building_id":1,"ap":60}`))
	request.Header.Set("X-Request-ID", "contribute-success")
	request.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	response = httptest.NewRecorder()
	logOutput = captureStdout(t, func() { handler.ServeHTTP(response, request) })
	if response.Code != http.StatusOK || !strings.Contains(logOutput, "user_id=1 action=contribute-construction outcome=success request_id=contribute-success") {
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
		if failureBody["ap"] != float64(maxAP-60) || failureBody["buildings"].([]any)[0].(map[string]any)["contributed_ap"] != float64(60) {
			t.Fatalf("rejected contribution changed state = %#v", failureBody)
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
	if body["ap"] != float64(maxAP-10) || len(body["crafting_recipes"].([]any)) != 1 {
		t.Fatalf("craft response state = %#v", body)
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
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).UnixNano(), identity.ID); err != nil {
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
	request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{}`))
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
	if body["ap"] != float64(maxAP-1) || body["error"] != nil {
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
		{name: "duplicate field", body: `{"resource_yield":1,"resource_yield":1}`, reason: convertReasonDuplicate},
		{name: "trailing value", body: `{}{}`, reason: convertReasonExtraValue},
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
	locationRequest := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{}`))
	locationRequest.Header.Set("X-Request-ID", "convert-location-request")
	locationRequest.AddCookie(&http.Cookie{Name: defaultSessionCookieName, Value: "session-secret"})
	locationResponse := httptest.NewRecorder()
	locationLog := captureStdout(t, func() { server.Routes().ServeHTTP(locationResponse, locationRequest) })
	if locationResponse.Code != http.StatusBadRequest || !strings.Contains(locationResponse.Body.String(), `"error":"conversion not found"`) {
		t.Fatalf("location convert response = %d: %s", locationResponse.Code, locationResponse.Body.String())
	}
	var locationBody map[string]any
	if err := json.Unmarshal(locationResponse.Body.Bytes(), &locationBody); err != nil {
		t.Fatal(err)
	}
	if resources := responseResourceQuantities(t, locationBody); len(resources) != 8 || resources["wood"] != 0 {
		t.Fatalf("location convert resources = %#v", resources)
	}
	if !strings.Contains(locationLog, "action=convert outcome=error reason="+convertReasonInvalidLocation+" request_id=convert-location-request") {
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
	itemRequest := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{}`))
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
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).UnixNano(), identity.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/actions/convert", strings.NewReader(`{}`))
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
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	location, ok := body["location"].(map[string]any)
	if !ok || location["id"] != "forest_edge" || body["ap"] != float64(maxAP-20) {
		t.Fatalf("move response = %#v", body)
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
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).UnixNano(), identity.ID); err != nil {
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
	if _, err := store.db.Exec("UPDATE player_ap SET full_timestamp = ? WHERE user_id = ?", now.Add(maxAP*time.Minute).UnixNano(), identity.ID); err != nil {
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
	if len(body) != 2 || body["error"] != ErrInsufficientAP.Error() || body["ap"] != float64(0) {
		t.Fatalf("insufficient AP JSON = %#v", body)
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
