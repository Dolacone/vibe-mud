package main

import "testing"

func TestLoadConfigRejectsIncompleteGoogleRuntime(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err == nil || cfg != (runtimeConfig{}) {
		t.Fatal("incomplete runtime configuration was accepted")
	}
}

func TestLoadConfigUsesFlySQLiteDefaults(t *testing.T) {
	values := map[string]string{"FRONTEND_URL": "https://game.example.test", "GOOGLE_CLIENT_ID": "id", "GOOGLE_CLIENT_SECRET": "secret", "GOOGLE_REDIRECT_URL": "https://api.example.test/auth/google/callback"}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabasePath != "/data/mud.db" || cfg.Port != "8080" || !cfg.CookieSecure {
		t.Fatalf("unexpected Fly defaults: %+v", cfg)
	}
}
