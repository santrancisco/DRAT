package score

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func setupTest() (r *github.Repository, u *github.User, c *github.Client, err error) {
	err = nil
	ctx := context.Background()
	c = github.NewClient(&http.Client{})
	if os.Getenv("GITHUB_AUTH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
		)

		tc := oauth2.NewClient(ctx, ts)
		c = github.NewClient(tc)
	}
	r, _, err = c.Repositories.Get(ctx, "google", "go-github")
	if err != nil {
		return nil, nil, nil, err
	}
	u, _, err = c.Users.Get(ctx, "santrancisco")
	if err != nil {
		return nil, nil, nil, err
	}
	return r, u, c, err
}

func TestRepoNew(t *testing.T) {
	t.Log("Test NewRepo")
	r, _, c, err := setupTest()
	if err != nil {
		t.Errorf("Setup for test failed:\n%v", err)
	}
	rs := NewGithubRepositoryScore(c, r)
	err = rs.Score(context.Background())
	if err != nil {
		t.Errorf("Scoring problem: %v", err)
	}
	// TODO: Write test cases.
}
