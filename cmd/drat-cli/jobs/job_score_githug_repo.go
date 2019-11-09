package jobs

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/santrancisco/drat/crawl"
	"github.com/santrancisco/drat/score"
	"github.com/santrancisco/cque"
)

const (
	KeyScoreGitHubRepo = "score_githug_repo"
)

// Example - we can wrap the other function
func ScoreGitHubRepo(logger *log.Logger, qc *cque.Client, j *cque.Job, appconfig map[string]interface{}) error {
	repojob := j.Args.(RepoJob)
	err := IsCrawlLimitReach(repojob, appconfig)
	if err != nil {
		return err
	}

	err = ScoreGitHubRepoFunc(logger, qc, j, appconfig)
	switch err {
	// Do something about error here, eg: in some case we could return ErrImmediateReschedule
	// We could also trap error code for rate limit reach?
	case nil:
		return nil
	default:
		logger.Print("[DEBUG] Error out in ScoreGithubRepo")
		return err

	}
}

// ScoreGitHubRepoFuncResult is the type of result that would return from our ScoreGithubRepo function.
type ScoreGitHubRepoFuncResult struct {
	ID                        string
	DependedOnBy              string
	Ownername                 string
	Name                      string
	URL                       string
	Dependencies              []string
	DependenciesCrawlProblems []crawl.Dependencyproblem
	RiskNotes                 map[string][]string
}

// For every result type, we will need to implement String() method to satisfy our Result interface.
func (sr *ScoreGitHubRepoFuncResult) String() string {
	b, err := json.Marshal(sr)
	if err != nil {
		return ""
	}
	return string(b)
}

func ScoreGitHubRepoFunc(logger *log.Logger, qc *cque.Client, j *cque.Job, appconfig map[string]interface{}) error {
	repojob := j.Args.(RepoJob)
	repofullurlparts := strings.Split(repojob.Fullname, "/")
	reponame := repofullurlparts[len(repofullurlparts)-1]
	owner := repofullurlparts[len(repofullurlparts)-2]
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s/%s", owner, reponame)))
	id := fmt.Sprintf("%x", h.Sum(nil))
	c, err := GetConn(appconfig)
	if err != nil {
		return err
	}
	repo, resp, err := c.Repositories.Get(context.Background(), owner, reponame)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to get information about repository %s/%s", owner, reponame)
	}
	rs := score.NewGithubRepositoryScore(c, repo)
	err = rs.Score(context.Background())
	if err != nil {
		return err
	}

	sr := ScoreGitHubRepoFuncResult{
		ID:                        id,
		DependedOnBy:              repojob.DependedOnBy,
		Ownername:                 *rs.R.Owner.Login,
		Name:                      *rs.R.Name,
		URL:                       *rs.R.URL,
		DependenciesCrawlProblems: []crawl.Dependencyproblem{},
		Dependencies:              []string{},
		RiskNotes:                 rs.RiskNote,
	}

	lst, dependenciesproblems := crawl.GithubDependencyCrawl(logger, c, repo, appconfig)
	sr.DependenciesCrawlProblems = dependenciesproblems
	sr.Dependencies = lst
	qc.Result <- cque.Result{
		JobType: j.Type,
		Result:  sr,
	}

	for _, v := range lst {
		if strings.Contains(v, "github.com") {
			logger.Printf("[INFO] Queue scoring job for \"%s\" found in repo %s/%s\n", v, *repo.Owner.Login, *repo.Name)
			qc.Enqueue(cque.Job{
				Type: KeyScoreGitHubRepo,
				Args: RepoJob{Fullname: v, DependedOnBy: id, Currentdepth: repojob.Currentdepth + 1},
			})
			continue
		}
		// If it does not match with any type of Scoring job, discard it
		logger.Printf("[INFO] No scoring job for \"%s\" found in repo %s/%s\n", v, *repo.Owner.Login, *repo.Name)
	}

	return nil
}
