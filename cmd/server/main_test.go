package main

import "testing"

func TestLoadConfigRejectsIncompleteGoogleRuntime(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err == nil || cfg != (runtimeConfig{}) {
		t.Fatal("incomplete runtime configuration was accepted")
	}
}

func TestLoadConfigUsesFlySQLiteDefaults(t *testing.T) {
	values := map[string]string{"FRONTEND_URL": "https://game.example.test", "GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "secret", "GOOGLE_REDIRECT_URL": "https://game.example.test/auth/google/callback", "COOKIE_SECURE": "false"}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabasePath != "/data/mud.db" || cfg.Port != "8080" || !cfg.CookieSecure {
		t.Fatalf("unexpected Fly defaults: %+v", cfg)
	}
}

func TestLoadConfigKeepsCookiesSecureForInvalidOverride(t *testing.T) {
	values := map[string]string{"FRONTEND_URL": "https://game.example.test", "GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "secret", "GOOGLE_REDIRECT_URL": "https://game.example.test/auth/google/callback", "COOKIE_SECURE": "invalid"}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CookieSecure {
		t.Fatal("invalid cookie security override disabled Secure")
	}
}

func TestLoadConfigRejectsMismatchedOrNonExactOAuthOrigins(t *testing.T) {
	base := map[string]string{"FRONTEND_URL": "https://game.example.test", "GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "secret", "GOOGLE_REDIRECT_URL": "https://game.example.test/auth/google/callback"}
	tests := map[string]struct {
		frontend string
		redirect string
	}{
		"different frontend origin": {frontend: "https://other.example.test", redirect: base["GOOGLE_REDIRECT_URL"]},
		"different callback origin": {frontend: base["FRONTEND_URL"], redirect: "https://other.example.test/auth/google/callback"},
		"callback path":             {frontend: base["FRONTEND_URL"], redirect: "https://game.example.test/auth/google/callback/extra"},
		"callback query":            {frontend: base["FRONTEND_URL"], redirect: "https://game.example.test/auth/google/callback?next=/"},
		"callback fragment":         {frontend: base["FRONTEND_URL"], redirect: "https://game.example.test/auth/google/callback#fragment"},
		"frontend path":             {frontend: "https://game.example.test/game", redirect: base["GOOGLE_REDIRECT_URL"]},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			values := map[string]string{"FRONTEND_URL": test.frontend, "GOOGLE_CLIENT_ID": base["GOOGLE_CLIENT_ID"], "GOOGLE_CLIENT_SECRET": base["GOOGLE_CLIENT_SECRET"], "GOOGLE_REDIRECT_URL": test.redirect}
			if _, err := loadConfig(func(key string) string { return values[key] }); err == nil {
				t.Fatal("invalid runtime origins were accepted")
			}
		})
	}
}

func TestLoadConfigNormalizesOriginTrailingSlash(t *testing.T) {
	values := map[string]string{"FRONTEND_URL": "https://GAME.example.test/", "GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "secret", "GOOGLE_REDIRECT_URL": "https://game.example.test/auth/google/callback"}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FrontendURL != "https://game.example.test" || cfg.RedirectURL != "https://game.example.test/auth/google/callback" {
		t.Fatalf("origins were not normalized: frontend=%q redirect=%q", cfg.FrontendURL, cfg.RedirectURL)
	}
}
