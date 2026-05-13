package store

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slipAPIResponse mirrors the FindByCommitsOutputBody JSON shape.
type slipAPIResponse struct {
	MatchedCommit string  `json:"matched_commit"`
	Slip          slipObj `json:"slip"`
}

type slipObj struct {
	CorrelationID string    `json:"correlation_id"`
	Repository    string    `json:"repository"`
	Branch        string    `json:"branch"`
	CommitSha     string    `json:"commit_sha"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *SlipAPIAdapter) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	adapter, err := NewSlipAPIAdapter(srv.URL, "test-key")
	require.NoError(t, err)
	return srv, adapter
}

func TestSlipAPIAdapter_FindByCommits_Success(t *testing.T) {
	_, adapter := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/slips/find-by-commits", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var got struct {
			Repository string   `json:"repository"`
			Commits    []string `json:"commits"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "org/repo", got.Repository)
		assert.Equal(t, []string{"abc123"}, got.Commits)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(slipAPIResponse{
			MatchedCommit: "abc123",
			Slip:          slipObj{CorrelationID: "test-correlation-id"},
		})
	})

	slip, matchedCommit, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	require.NotNil(t, slip)
	assert.Equal(t, "test-correlation-id", slip.CorrelationID)
	assert.Equal(t, "abc123", matchedCommit)
}

func TestSlipAPIAdapter_FindByCommits_Unauthorized_SurfacesAuthHint(t *testing.T) {
	_, adapter := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"title":  "Unauthorized",
			"detail": "invalid bearer token",
		})
	})

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SLIPPY_API_KEY")
	assert.Contains(t, err.Error(), "invalid bearer token")
}

func TestSlipAPIAdapter_FindByCommits_NotFound(t *testing.T) {
	_, adapter := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	slip, matchedCommit, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	assert.Nil(t, slip)
	assert.Equal(t, "", matchedCommit)
}

func TestSlipAPIAdapter_FindByCommits_ServerError(t *testing.T) {
	_, adapter := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	slip, matchedCommit, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
	assert.Nil(t, slip)
	assert.Equal(t, "", matchedCommit)
}

func TestSlipAPIAdapter_FindByCommits_EmptyBody(t *testing.T) {
	_, adapter := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		// Return 200 with empty body — should be treated as an error.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})
	require.Error(t, err)
}

func TestSlipAPIAdapter_Close(t *testing.T) {
	_, adapter := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	assert.NoError(t, adapter.Close())
}
