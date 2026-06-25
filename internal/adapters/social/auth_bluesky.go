// Package social — Bluesky authenticated session.
// app.bsky.feed.searchPosts on public.api.bsky.app now rejects unauthenticated
// requests with HTTP 403; an authenticated session JWT is required. This file
// exchanges a handle + app password for an accessJwt via createSession.
package social

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// blueskySessionURL is the atproto createSession endpoint (entryway host).
const blueskySessionURL = "https://bsky.social/xrpc/com.atproto.server.createSession"

// createBlueskySession exchanges identifier + app password for an accessJwt.
//
// ponytail: fetched once at construction, no refresh. accessJwt expires (~2h),
// so a long-lived process must restart to re-auth; add refreshJwt handling here
// if session lifetime becomes a problem.
func createBlueskySession(ctx context.Context, client *http.Client, identifier, appPassword string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"identifier": identifier,
		"password":   appPassword,
	})
	if err != nil {
		return "", fmt.Errorf("social: marshal createSession payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, blueskySessionURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("social: build createSession request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("social: createSession request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("social: createSession HTTP %d: %s", resp.StatusCode, string(body))
	}

	var session struct {
		AccessJwt string `json:"accessJwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("social: decode createSession response: %w", err)
	}
	if session.AccessJwt == "" {
		return "", fmt.Errorf("social: createSession returned empty accessJwt")
	}
	return session.AccessJwt, nil
}
