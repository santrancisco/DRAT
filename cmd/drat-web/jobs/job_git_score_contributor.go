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
	KeyGitScoreContributor = "git_score_contributor"
)

type Contributor struct {
	C      *github.Contributor
	RepoID int
	Score  int
}

func GitScoreContributor(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx) error {
	ctx := context.Background()
	client := github.NewClient(&http.Client{})
	if os.Getenv("GITHUB_AUTH_TOKEN") != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
		)

		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	}

	// We do our scoring here and set scoring
	var contrib Contributor
	err := json.Unmarshal(job.Args, &contrib)
	if err != nil {
		return err
	}

	// Simulating this jobfunc for testing purpose
	time.Sleep(2 * time.Second)
	scoreGitContributor(client, logger, tx, &contrib)
	// TODO: Write a jobfunc to rate contributor base on the following:
	//  - Account creation date
	//  - Number of follower
	//  - Number of subscriptions , starred project (help to identify bot-user)
	//  - List repository and the last updated (eg https://api.github.com/users/santrancisco/repos?sort=created)
	//  - Frequency of PushEvent (https://api.github.com/users/santrancisco/events)

	// We will store the final result with contrib.Scoring is scored

	err = recordGitContributor(qc, logger, job, tx, contrib)
	if err != nil {
		return err
	}
	return nil
}

func scoreGitContributor(client *github.Client, logger *log.Logger, tx *pgx.Tx, contrib *Contributor) {

}

func recordGitContributor(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx, contrib Contributor) error {
	var id int
	var lastUpdate time.Time
	c, err := json.Marshal(*contrib.C)
	if err != nil {
		return err
	}
	// Check if the contributor is already in our database, otherwise INSERT it
	err = tx.QueryRow("SELECT id, last_updated FROM git_contributors WHERE id = $1 FOR UPDATE", *contrib.C.ID).Scan(&id, &lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows { // If we receive no record, create a new one
			sqlInsertOrg := `INSERT INTO git_contributors
			(id, login, raw_description, score, avatar_url)
			VALUES
			($1::integer, $2::text, coalesce($3::json, '[]'::json), coalesce($4::smallint,100::smallint),  coalesce($5::text,'https://avatars3.githubusercontent.com/u/0?v=4'::text))`
			_, err = tx.Exec(sqlInsertOrg,
				*contrib.C.ID,
				*contrib.C.Login,
				c,
				contrib.Score,
				*contrib.C.AvatarURL)
			if err != nil {
				return err
			}
			recordContributorRelation(logger, job, tx, contrib)
			if err != nil {
				return err
			}
			return nil // We created new organisation into our db and return without error.
		}
		return err // Something else broke, we return the error.
	}
	// If the record does exist, we have already locked the row using SELECT...FOR UPDATE, we will now updating it and commit it later

	sqlUpdateOrg := `UPDATE git_contributors SET
			login = $1::text,
			raw_description = coalesce($2::json, '[]'::json),
			score   = coalesce($3::smallint,100::smallint),
			avatar_url   = coalesce($4::text,'https://avatars3.githubusercontent.com/u/0?v=4'),
			last_updated = now()
			WHERE id = $5::integer`
	_, err = tx.Exec(sqlUpdateOrg,
		*contrib.C.Login,
		c,
		contrib.Score,
		*contrib.C.AvatarURL,
		*contrib.C.ID)
	if err != nil {
		return err
	}

	recordContributorRelation(logger, job, tx, contrib)
	if err != nil {
		return err
	}
	return nil
}

func recordContributorRelation(logger *log.Logger, job *que.Job, tx *pgx.Tx, contrib Contributor) error {
	var lastUpdate time.Time
	// Check if the repository is already in our database
	err := tx.QueryRow("SELECT added_at FROM git_repository_to_contributor WHERE repository_id = $1 AND contributor_id = $2", contrib.RepoID, *contrib.C.ID).Scan(&lastUpdate)
	if err != nil {
		if err == pgx.ErrNoRows { // If we receive no record, create a new one
			sqlInsertOrg := `INSERT INTO git_repository_to_contributor
			(repository_id, contributor_id)
			VALUES
			($1::integer, $2::integer)`
			_, err = tx.Exec(sqlInsertOrg,
				contrib.RepoID,
				*contrib.C.ID)
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
