package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepository(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/example/repo", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(&gh.Repository{Name: gh.String("repo")})
	}))
	defer server.Close()

	client := gh.NewClient(server.Client())
	client.BaseURL = mustParseURL(t, server.URL+"/")

	repoClient := &Client{client: client}
	repo, err := repoClient.GetRepository(context.Background(), "example", "repo")
	require.NoError(t, err)
	assert.Equal(t, "repo", repo.GetName())
}

func TestCreateBranch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/example/repo/git/refs", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := gh.NewClient(server.Client())
	client.BaseURL = mustParseURL(t, server.URL+"/")

	repoClient := &Client{client: client}
	require.NoError(t, repoClient.CreateBranch(context.Background(), "example", "repo", "feature", "abc123"))
}

func TestClientErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("get repository", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := gh.NewClient(server.Client())
		client.BaseURL = mustParseURL(t, server.URL+"/")
		repoClient := &Client{client: client}

		_, err := repoClient.GetRepository(context.Background(), "example", "repo")
		require.Error(t, err)
		assert.ErrorContains(t, err, "get github repository")
	})

	t.Run("create branch", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := gh.NewClient(server.Client())
		client.BaseURL = mustParseURL(t, server.URL+"/")
		repoClient := &Client{client: client}

		err := repoClient.CreateBranch(context.Background(), "example", "repo", "feature", "abc123")
		require.Error(t, err)
		assert.ErrorContains(t, err, "create github branch")
	})
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	withoutToken := NewClient(context.Background(), "")
	require.NotNil(t, withoutToken)
	require.NotNil(t, withoutToken.client)

	withToken := NewClient(context.Background(), "token")
	require.NotNil(t, withToken)
	require.NotNil(t, withToken.client)
}

func mustParseURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	require.NoError(t, err)
	return parsed
}
