package authapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	t.Helper()
	store, _ := newTestStore(t)
	server, err := NewServer(store, provider, Config{
		FrontendURL:     "https://game.example.test",
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

func loginState(t *testing.T, handler http.Handler) string {
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
	return location.Query().Get("state")
}

func TestLoginRedirectIncludesStateNonceAndPKCE(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{}
	server, store := newTestServer(t, provider, &now)
	state := loginState(t, server.Routes())
	if state == "" || provider.authorization.nonce == "" || provider.authorization.challenge == "" {
		t.Fatal("login redirect did not receive state, nonce, and PKCE challenge")
	}
	attempt, err := store.ConsumeOAuthAttempt(state)
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
	state := loginState(t, handler)
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=code-secret", nil)
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

	expiredState := loginState(t, handler)
	now = now.Add(11 * time.Minute)
	expired := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(expiredState)+"&code=another-code", nil)
	expiredResponse := httptest.NewRecorder()
	handler.ServeHTTP(expiredResponse, expired)
	if expiredResponse.Code != http.StatusBadRequest || provider.exchangeCalls != 1 {
		t.Fatalf("expired callback status=%d exchange calls=%d; expired state reached provider", expiredResponse.Code, provider.exchangeCalls)
	}
}

func TestCallbackSetsConstrainedSessionCookieAndRedirectsFrontend(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	provider := &fakeProvider{identity: ProviderIdentity{
		Issuer: "https://accounts.google.com", Subject: "subject-1", Email: "person@example.com", DisplayName: "Person", Nonce: "placeholder",
	}}
	server, store := newTestServer(t, provider, &now)
	handler := server.Routes()
	state := loginState(t, handler)
	provider.identity.Nonce = provider.authorization.nonce
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=authorization-code-secret", nil)
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
	if len(cookies) != 1 {
		t.Fatalf("callback set %d cookies, want one", len(cookies))
	}
	cookie := cookies[0]
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
	state := loginState(t, handler)
	provider.identity.Nonce = provider.authorization.nonce
	callback := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=authorization-code-secret", nil)
	callbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackResponse, callback)
	sessionCookie := callbackResponse.Result().Cookies()[0]

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
