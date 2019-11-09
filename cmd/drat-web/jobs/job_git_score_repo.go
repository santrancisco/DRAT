package jobs

import (
	"context"
	"encoding/json"
	"fmt"
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
	KeyGitScoreRepository = "git_score_repository"
)

type Repository struct {
	R        *github.Repository
	OrgID    int
	ParentID int
	Score    int
}

// GitScoreRepository: Our scoring function
func GitScoreRepository(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx) error {
	ctx := context.Background()
	client := github.NewClient(&http.Client{})
	if os.Getenv("GITHUB_AUTH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
		)

		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	}
	_ = client
	// We do our scoring here and set scoring
	var repo Repository
	err := json.Unmarshal(job.Args, &repo)
	if err != nil {
		return err
	}
	// Simulating this jobfunc for testing purpose
	time.Sleep(5 * time.Second)
	scoreGitRepository(client, logger, tx, &repo)
	// TODO: Write a jobfunc to score the credibility of a repository base on:
	// - The number of stars (there is no count api so we will have to use pagination to query the stargazer api which can be huge so we will start with 20 --> 50 --> 100 --> bail at anything above 200 )
	// - The number of watcher (/subscribers)
	// - The last Pushevent?
	// - Number of commit?
	// - Number of issues?

	// We will store the final result with repo.Scoring is scored

	err = recordGitRepository(qc, logger, job, tx, repo)
	if err != nil {
		return err
	}
	return nil
}

func scoreGitRepository(client *github.Client, logger *log.Logger, tx *pgx.Tx, repo *Repository) {

}

func recordGitRepository(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx, repo Repository) error {
	var id int
	var lastUpdate time.Time
	r, err := json.Marshal(*repo.R)
	if err != nil {
		return err
	}
	// Check if the repository is already in our database, otherwise INSERT it
	err = tx.QueryRow("SELECT id, last_updated FROM git_repositories WHERE id = $1 FOR UPDATE", *repo.R.ID).Scan(&id, &lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows { // If we receive no record, create a new one
			sqlInsertOrg := `INSERT INTO git_repositories
			(id, full_name, raw_description, score)
			VALUES
			($1::integer, $2::text, coalesce($3::json, '[]'::json), coalesce($4::smallint,100::smallint) )`
			_, err = tx.Exec(sqlInsertOrg,
				*repo.R.ID,
				*repo.R.FullName,
				r,
				repo.Score)
			if err != nil {
				return err
			}
			err = recordGitRelation(logger, job, tx, repo)
			if err != nil {
				return err
			}
			return nil // We created new organisation into our db and return without error.
		}
		return err // Something else broke, we return the error.
	}
	// If the record does exist, we have already locked the row using SELECT...FOR UPDATE, we will now updating it and commit it later

	sqlUpdateOrg := `UPDATE git_repositories SET
			full_name = $1::text,
			raw_description = coalesce($2::json, '[]'::json),
			score   = coalesce($3::smallint,100::smallint),
			last_updated = now()
			WHERE id = $4::integer`
	_, err = tx.Exec(sqlUpdateOrg,
		*repo.R.FullName,
		r,
		repo.Score,
		*repo.R.ID)
	if err != nil {
		return err
	}

	err = recordGitRelation(logger, job, tx, repo)
	if err != nil {
		return err
	}
	return nil
}

func recordGitRelation(logger *log.Logger, job *que.Job, tx *pgx.Tx, repo Repository) error {
	// If this repository was acquired from crawling
	fmt.Println(repo.OrgID)
	if repo.OrgID != -1 {
		// Add org to repo relationship
		err := recordGitOrgToRepository(logger, job, tx, repo)
		if err != nil {
			return err
		}
	}
	fmt.Println(repo.ParentID)
	if repo.ParentID != -1 {
		// Add relationship between repository and its dependencies
		err := recordGitRepositoryToDependency(logger, job, tx, repo)
		if err != nil {
			return err
		}
	}
	return nil
}

func recordGitOrgToRepository(logger *log.Logger, job *que.Job, tx *pgx.Tx, repo Repository) error {
	var lastUpdate time.Time
	// Check if the repository is already in our database
	err := tx.QueryRow("SELECT added_at FROM git_org_to_repository WHERE organisation_id = $1 AND repository_id = $2", repo.OrgID, *repo.R.ID).Scan(&lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows { // If we receive no record, create a new one
			sqlInsertOrg := `INSERT INTO git_org_to_repository
			(organisation_id, repository_id)
			VALUES
			($1::integer, $2::integer)`
			_, err = tx.Exec(sqlInsertOrg,
				repo.OrgID,
				*repo.R.ID)
			if err != nil {
				return err
			}
			return nil // We created new organisation into our db and return without error.
		}
		return err // Something else broke, we return the error.
	}
	// If the record does exist, we have already locked the row using SELECT...FOR UPDATE, we will now updating it and commit it later
	return nil
}

func recordGitRepositoryToDependency(logger *log.Logger, job *que.Job, tx *pgx.Tx, repo Repository) error {
	var lastUpdate time.Time
	// Check if the repository is already in our database
	err := tx.QueryRow("SELECT added_at FROM git_repository_to_dependency WHERE repository_id = $1 AND dependency_id = $2", repo.ParentID, *repo.R.ID).Scan(&lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows { // If we receive no record, create a new one
			sqlInsertOrg := `INSERT INTO git_repository_to_dependency
			(repository_id, dependency_id)
			VALUES
			($1::integer, $2::integer)`
			_, err = tx.Exec(sqlInsertOrg,
				repo.ParentID,
				*repo.R.ID)
			if err != nil {
				return err
			}
			return nil // We created new organisation into our db and return without error.
		}
		return err // Something else broke, we return the error.
	}
	// If the record does exist, we have already locked the row using SELECT...FOR UPDATE, we will now updating it and commit it later
	return nil
}
