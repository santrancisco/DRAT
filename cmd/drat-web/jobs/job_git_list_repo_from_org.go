package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	que "github.com/bgentry/que-go"
	github "github.com/google/go-github/github"
	"github.com/jackc/pgx"
	"golang.org/x/oauth2"
)

type GitOrg struct {
	Name string
}

const (
	KeyGitListRepoFromOrg        = "git_list_repo_from_org"
	KeyGitGetDependenciesPrepend = "git_get_dependencies_"
)

func GitListRepoFromOrg(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx) error {
	var org GitOrg
	err := json.Unmarshal(job.Args, &org)
	// Do local first for testing.
	//err := json.Unmarshal([]byte(Orgtest), &org)
	if err != nil {
		return err
	}
	SupportedLanguage := strings.Split(os.Getenv("SUPPORTED_LANGUAGE"), ",")
	ctx := context.Background()
	client := github.NewClient(&http.Client{})
	if os.Getenv("GITHUB_AUTH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
		)

		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	}
	orgdetails, _, err := client.Organizations.Get(ctx, org.Name)
	if err != nil {
		return err
	}
	err = recordGitOrg(logger, job, tx, orgdetails)
	if err != nil {
		logger.Print("Failed to add %s to the database", org.Name)
		return err
	}
	var allRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org.Name, opt)
		if err != nil {
			return err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	// Sending repository scoring job to the queue
	for _, r := range allRepos {
		jobarguments := Repository{R: r, OrgID: int(*orgdetails.ID), ParentID: -1, Score: 100} // Default all repository start with 100 credit points
		jarg, err := json.Marshal(jobarguments)
		if err != nil {
			return err
		}
		if r.Language != nil {
			for _, i := range SupportedLanguage { // If we support this language - Add a new job
				if strings.TrimSpace(strings.ToLower(strings.TrimSpace(i))) == strings.ToLower(strings.TrimSpace(*r.Language)) {
					DynamicKey := strings.ToLower(fmt.Sprintf("%s%s", KeyGitGetDependenciesPrepend, strings.TrimSpace(*r.Language)))
					// For each repository in an organisation we are spawning 3 jobs:

					// Depdency crawler job
					err = qc.EnqueueInTx(&que.Job{
						Type: DynamicKey, // Adding dependency crawler job
						Args: jarg,
					}, tx)
					logger.Printf("Queueing new job: %s - %s", DynamicKey, *r.Name)

					// Repository scoring job
					err = qc.EnqueueInTx(&que.Job{
						Type: KeyGitScoreRepository, // Adding repository scoring job
						Args: jarg,
					}, tx)
					logger.Printf("Queueing new job: %s - %s", KeyGitScoreRepository, *r.Name)

					// Adding a job to get the list of contributors to the queue
					err = qc.EnqueueInTx(&que.Job{
						Type: KeyGitListContributorFromRepo, // Adding dependency crawler job
						Args: jarg,
					}, tx)
					logger.Printf("Queueing new job: %s - %s", DynamicKey, *r.Name)
				}
			}
			logger.Printf(fmt.Sprintf("Language not yet support for %s - %s", *r.Name, *r.Language))
			_, err := tx.Exec("INSERT INTO error_log (error) VALUES ($1)", fmt.Sprintf("Language not yet support for %s - %s", *r.Name, *r.Language))
			if err != nil {
				return err
			}
		} else {
			_, err := tx.Exec("INSERT INTO error_log (error) VALUES ($1)", fmt.Sprintf("Unknown Language for repository %s", *r.Name))
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func recordGitOrg(logger *log.Logger, job *que.Job, tx *pgx.Tx, org *github.Organization) error {
	var id int
	var lastUpdate time.Time
	o, err := json.Marshal(*org)
	if err != nil {
		return err
	}
	err = tx.QueryRow("SELECT id, last_updated FROM git_organisations WHERE id = $1 FOR UPDATE", *org.ID).Scan(&id, &lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows { // If we receive no record, create a new one
			sqlInsertOrg := `INSERT INTO git_organisations 
			(id, login, name, avatar_url, raw_description) 
			VALUES 
			($1::integer, $2::text, $3::text, coalesce($4::text,''::text), coalesce($5::json, '[]'::json))`
			_, err = tx.Exec(sqlInsertOrg,
				*org.ID,
				*org.Login,
				*org.Name,
				*org.AvatarURL,
				o)
			if err != nil {
				return err
			}
			return nil // We created new organisation into our db and return without error.
		}
		return err // Something else broke, we return the error.
	}
	// If the record does exist, we have already locked the row using SELECT...FOR UPDATE, we will now updating it and commit it later

	sqlUpdateOrg := `UPDATE git_organisations SET 
			login = $1::text, 
			name  = $2::text,
			avatar_url = coalesce($3::text,''::text),
			raw_description = coalesce($4::json, '[]'::json),
			last_updated = now()
			WHERE id = $5::integer`
	_, err = tx.Exec(sqlUpdateOrg,
		*org.Login,
		*org.Name,
		*org.AvatarURL,
		o,
		*org.ID)
	if err != nil {
		return err
	}
	return nil
}
