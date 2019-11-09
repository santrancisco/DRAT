package jobs

import (
	"context"
	"fmt"
	"net/http"

	github "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type RepoJob struct {
	Fullname     string
	DependedOnBy string
	Currentdepth int
}

// Check if crawl limit is reached, if it does, return a customed errorcode.
func IsCrawlLimitReach(repojob RepoJob, appconfig map[string]interface{}) error {
	if repojob.Currentdepth < appconfig["depth"].(int) {
		return nil
	}
	return fmt.Errorf("Crawl limit reachs at %s", repojob.Fullname)
}
func GetConn(config map[string]interface{}) (c *github.Client, err error) {
	err = nil
	ctx := context.Background()
	c = github.NewClient(&http.Client{})
	if config["gitauthtoken"].(string) != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: config["gitauthtoken"].(string)},
		)
		tc := oauth2.NewClient(ctx, ts)
		c = github.NewClient(tc)
	}
	return c, nil
}
