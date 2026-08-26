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
	"strings"
	"testing"
	"time"
)

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
	if len(meBody) != 6 || meBody["id"] != float64(identity.ID) || meBody["display_name"] != "Person" || meBody["email"] != "person@example.com" || meBody["ap"] != float64(maxAP) {
		t.Fatalf("GET /api/me JSON = %#v", meBody)
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
	request := httptest.NewRequest(http.MethodPost, "/api/actions/attack", nil)
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
