package authapi

import (
	"context"
	"testing"
)

func TestNewGoogleProviderRejectsIncompleteConfigBeforeDiscovery(t *testing.T) {
	if _, err := NewGoogleProvider(context.Background(), GoogleConfig{}); err == nil {
		t.Fatal("incomplete Google configuration was accepted")
	}
}
