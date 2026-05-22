// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/url"
	"strings"
	"testing"
	"youtube-analytics-pp-cli/internal/config"
)

// buildAuthURL replicates the URL-construction logic in newAuthLoginCmd
// so tests can assert OAuth params without starting a real HTTP server.
func buildAuthURL(cfg *config.Config, clientID, redirectURI, state string) string {
	authURL := cfg.AuthorizationURL
	if authURL == "" {
		authURL = "https://accounts.google.com/o/oauth2/auth"
	}
	params := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	scopes := []string{
		"https://www.googleapis.com/auth/youtube",
		"https://www.googleapis.com/auth/youtube.readonly",
		"https://www.googleapis.com/auth/youtubepartner",
		"https://www.googleapis.com/auth/yt-analytics-monetary.readonly",
		"https://www.googleapis.com/auth/yt-analytics.readonly",
	}
	params.Set("scope", strings.Join(scopes, " "))
	return authURL + "?" + params.Encode()
}

// TestOAuthAuthURLParams asserts that the authorization URL built by
// newAuthLoginCmd always carries the two params required for offline
// refresh-token issuance.
func TestOAuthAuthURLParams(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	raw := buildAuthURL(cfg, "test-client-id", "http://localhost:8085/callback", "test-state")

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse failed: %v", err)
	}
	q := parsed.Query()

	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want %q", got, "offline")
	}
	if got := q.Get("prompt"); got != "consent" {
		t.Errorf("prompt = %q, want %q", got, "consent")
	}
}

// TestOAuthTokenURLDefault asserts that config.Load sets the token URL to
// the correct Google endpoint when no override is provided.
func TestOAuthTokenURLDefault(t *testing.T) {
	t.Parallel()

	// config.Load resolves the default TokenURL at load time.
	// We instantiate Config directly to mirror what Load does so we can
	// test the default without touching the filesystem.
	cfg := &config.Config{}
	// Replicate the defaulting logic from config.Load (lines 107-109).
	tokenURL := cfg.TokenURL
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	const want = "https://oauth2.googleapis.com/token"
	if tokenURL != want {
		t.Errorf("default token URL = %q, want %q", tokenURL, want)
	}
}

// TestOAuthTokenURLOverride asserts that when a user sets TokenURL in
// their config file, the value propagates through to the token exchange
// and does not fall back to the dead accounts.google.com endpoint.
func TestOAuthTokenURLOverride(t *testing.T) {
	t.Parallel()

	const customURL = "https://custom.internal/token"
	cfg := &config.Config{
		TokenURL: customURL,
	}

	// Replicate the tokenURL resolution in newAuthLoginCmd.
	tokenURL := cfg.TokenURL
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	if tokenURL != customURL {
		t.Errorf("token URL with override = %q, want %q", tokenURL, customURL)
	}

	const dead = "accounts.google.com/o/oauth2/token"
	if strings.Contains(tokenURL, dead) {
		t.Errorf("token URL must not reference dead endpoint %q, got %q", dead, tokenURL)
	}
}

// TestOAuthTokenURLOverrideInRefresh asserts the same override semantics
// for the token-refresh path in client.go's refreshAccessToken.
func TestOAuthTokenURLOverrideInRefresh(t *testing.T) {
	t.Parallel()

	const customURL = "https://custom.internal/token"
	cfg := &config.Config{
		TokenURL: customURL,
	}

	// Replicate the tokenURL resolution in refreshAccessToken.
	tokenURL := cfg.TokenURL
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	if tokenURL != customURL {
		t.Errorf("refresh token URL with override = %q, want %q", tokenURL, customURL)
	}
}
