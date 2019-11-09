package jobs

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	que "github.com/bgentry/que-go"
	github "github.com/google/go-github/github"
	"github.com/jackc/pgx"
	"golang.org/x/oauth2"
)

const (
	KeyGitListContributorFromRepo = "git_list_contributor_from_repo"
)

// GitListContributorFromRepo is a jobfunc to acquire the list of contributors from a repository.
func GitListContributorFromRepo(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx) error {
	// Prepare a github API client
	ctx := context.Background()
	client := github.NewClient(&http.Client{})
	if os.Getenv("GITHUB_AUTH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
		)

		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	}
	var repo Repository
	err := json.Unmarshal(job.Args, &repo)
	if err != nil {
		return err
	}

	var allContributors []*github.Contributor
	opt := &github.ListContributorsOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	for {
		contributors, resp, err := client.Repositories.ListContributors(ctx, *repo.R.Owner.Login, *repo.R.Name, &github.ListContributorsOptions{})
		if err != nil {
			return err
		}
		allContributors = append(allContributors, contributors...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	// For every contributor, `we are queueing a new job to store the record and score the profile
	for _, c := range allContributors {
		jobarguments := Contributor{C: c, RepoID: int(*repo.R.ID), Score: 100} // Default all repository start with 100 credit points
		jarg, err := json.Marshal(jobarguments)
		if err != nil {
			return err
		}
		// Depdency crawler job
		err = qc.EnqueueInTx(&que.Job{
			Type: KeyGitScoreContributor, // Adding dependency crawler job
			Args: jarg,
		}, tx)
		logger.Printf("Queueing new job: %s - %s", KeyGitScoreContributor, *c.Login)

	}
	// This was for simulation purpose
	time.Sleep(2 * time.Second)
	return nil
}
