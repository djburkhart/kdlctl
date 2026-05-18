package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client *gh.Client
}

func NewClient(ctx context.Context, token string) *Client {
	if token == "" {
		return &Client{client: gh.NewClient(http.DefaultClient)}
	}

	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	return &Client{client: gh.NewClient(httpClient)}
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*gh.Repository, error) {
	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get github repository: %w", err)
	}

	return repository, nil
}

func (c *Client) CreateBranch(ctx context.Context, owner, repo, branch, sha string) error {
	_, _, err := c.client.Git.CreateRef(ctx, owner, repo, &gh.Reference{
		Ref: gh.String("refs/heads/" + branch),
		Object: &gh.GitObject{
			SHA: gh.String(sha),
		},
	})
	if err != nil {
		return fmt.Errorf("create github branch: %w", err)
	}

	return nil
}
