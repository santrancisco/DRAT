package score

import (
	"context"
	"fmt"
	"time"

	github "github.com/google/go-github/github"
)

type GithubRepositoryScore struct {
	Weight     GithubScoringWeight
	R          *github.Repository
	c          *github.Client
	fullname   string
	RiskNote   map[string][]string
	TotalScore int
}
type GithubScoringWeight struct {
	OwnerCountW       int
	ContributorCountW int
	LastUpdateW       int
	StarCountW        int
	WatcherCountW     int
}

var defaultGithubScoringWeight = GithubScoringWeight{
	OwnerCountW:       10,
	ContributorCountW: 10,
	LastUpdateW:       10,
	StarCountW:        10,
	WatcherCountW:     10,
}

// TODO: Write an init function to check for scoring weight configuration in a json or yml file and overwrite the default values.
// func init() {
// 	Check if score.config exist
//  Load scoring weight to it
// }

func NewGithubRepositoryScore(c *github.Client, r *github.Repository) *GithubRepositoryScore {
	rs := &GithubRepositoryScore{}
	rs.Weight = defaultGithubScoringWeight
	rs.R = r
	rs.c = c
	rs.fullname = "github.com/" + *r.Owner.Login + "/" + *r.Name
	rs.TotalScore = 100
	rs.RiskNote = map[string][]string{"RISK": []string{}, "GOOD": []string{}, "INFO": []string{}}
	return rs
}

func (rs *GithubRepositoryScore) Score(ctx context.Context) error {
	//var err error

	// fmt.Printf("Owner Type: %v\n", *rs.R.Owner.Type) // This can either be Organisation or User  - Org is more trust worthy

	// branches, _, err := rs.c.Repositories.ListBranches(ctx, *rs.R.Owner.Login, *rs.R.Name, &github.ListOptions{})
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("Number of branches: %v\n", len(branches))
	// rs.c.Repositories.List

	// issues, resp, err := rs.c.Issues.ListByRepo(ctx, *rs.R.Owner.Login, *rs.R.Name, &github.IssueListByRepoOptions{})
	// if err != nil {
	// 	return err
	// }
	// if resp.StatusCode != 200 {
	// 	return errors.New(string(resp.Body.Read(resp.ContentLength)))
	// }

	if *rs.R.Owner.Type != "Organization" {
		rs.RiskNote["INFO"] = append(rs.RiskNote["INFO"], fmt.Sprintf("Repository is not managed under an organisation"))
	}

	contributors, _, err := rs.c.Repositories.ListContributors(ctx, *rs.R.Owner.Login, *rs.R.Name, &github.ListContributorsOptions{ListOptions: github.ListOptions{PerPage: 100}})
	if err != nil {
		return err
	}
	if len(contributors) < 3 {
		rs.RiskNote["RISK"] = append(rs.RiskNote["RISK"], fmt.Sprintf("[LOW] Size of collaborator for the repository is %d which is less than 3", len(contributors)))
	}

	if time.Since(rs.R.GetPushedAt().Time) > (time.Duration(24*365) * time.Hour) {
		rs.RiskNote["RISK"] = append(rs.RiskNote["RISK"], fmt.Sprintf("[HIGH] Repository has not been updated for a year"))
	}

	if time.Since(rs.R.GetCreatedAt().Time) < (time.Duration(24*120) * time.Hour) {
		rs.RiskNote["RISK"] = append(rs.RiskNote["RISK"], fmt.Sprintf("[LOW] Repository is young and only been created for less than 120 days"))
	}
	if rs.R.License.GetKey() == "" {
		rs.RiskNote["RISK"] = append(rs.RiskNote["RISK"], fmt.Sprintf("[LOW] Repository does not have a license attached to it"))
	}

	if *rs.R.Fork {
		rs.RiskNote["RISK"] = append(rs.RiskNote["RISK"], fmt.Sprintf("[LOW] This repository was forked from %s", *rs.R.Parent.FullName))
	}

	forks, _, err := rs.c.Repositories.ListForks(ctx, *rs.R.Owner.Login, *rs.R.Name, &github.RepositoryListForksOptions{ListOptions: github.ListOptions{PerPage: 100}})
	if err != nil {
		return err
	}
	if len(forks) > 10 {
		rs.RiskNote["GOOD"] = append(rs.RiskNote["GOOD"], fmt.Sprintf("[GOOD] Repository has been forked %d times", len(forks)))
	}

	if rs.R.GetStargazersCount() > 50 {
		rs.RiskNote["GOOD"] = append(rs.RiskNote["GOOD"], fmt.Sprintf("[GOOD] Repository has been stared %d times", rs.R.GetStargazersCount()))
	}

	if rs.R.GetWatchersCount() > 50 {
		rs.RiskNote["GOOD"] = append(rs.RiskNote["GOOD"], fmt.Sprintf("[GOOD] Repository is being watched by %d people", rs.R.GetWatchersCount()))
	}

	if *rs.R.HasWiki {
		rs.RiskNote["GOOD"] = append(rs.RiskNote["GOOD"], fmt.Sprintf("[GOOD] Repository has a wiki"))
	}

	//Contributor stats
	// contributorStat, res, err := rs.c.Repositories.ListContributorsStats(ctx, *rs.R.Owner.Login, *rs.R.Name)
	// if err != nil {
	// 	return err
	// }
	// if res.StatusCode == 202 {
	// 	// Contributor stat require 2 operation at first run as it is an expensive operation, we can wait here until the data come back or move on and check back later
	// }

	// Latest release - does not work in many cases. we need to check Tag instead.
	// release, _, err := rs.c.Repositories.GetLatestRelease(ctx, *rs.R.Owner.Login, *rs.R.Name)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("Test%v", release)

	return nil
}
