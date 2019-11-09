package db

import (
	"errors"
	"sync"

	cfenv "github.com/cloudfoundry-community/go-cfenv"

	"github.com/bgentry/que-go"
	"github.com/jackc/pgx"
)

// Return a database object, using the CloudFoundry environment data
// This function retrieves the credential and the db conenction from VCAP_SERVICES environment variable
func postgresCredsFromCF() (map[string]interface{}, error) {
	appEnv, err := cfenv.Current()
	if err != nil {
		return nil, err
	}

	dbEnv, err := appEnv.Services.WithTag("postgres")
	if err != nil {
		return nil, err
	}

	if len(dbEnv) != 1 {
		return nil, errors.New("expecting 1 database")
	}

	return dbEnv[0].Credentials, nil
}

type DBInitter struct {
	InitSQL            string
	PreparedStatements map[string]string
	OtherStatements    func(*pgx.Conn) error

	// Clearly this won't stop other instances in a race condition, but should at least stop ourselves from hammering ourselves unnecessarily
	runMutex   sync.Mutex
	runAlready bool
}

func (dbi *DBInitter) ensureInitDone(c *pgx.Conn) error {
	dbi.runMutex.Lock()
	defer dbi.runMutex.Unlock()

	if dbi.runAlready {
		return nil
	}

	_, err := c.Exec(dbi.InitSQL)
	if err != nil {
		return err
	}

	dbi.runAlready = true
	return nil
}

func (dbi *DBInitter) AfterConnect(c *pgx.Conn) error {
	if dbi.InitSQL != "" {
		err := dbi.ensureInitDone(c)
		if err != nil {
			return err
		}
	}

	if dbi.OtherStatements != nil {
		err := dbi.OtherStatements(c)
		if err != nil {
			return err
		}
	}

	if dbi.PreparedStatements != nil {
		for n, sql := range dbi.PreparedStatements {
			_, err := c.Prepare(n, sql)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// OtherStatements is part of que library which prepare all SQL statements related to managing jobs for performance
// List of it can be found here: https://github.com/bgentry/que-go/blob/04623be4201ec7abb8d6cd0f172a4c612e216cc4/que.go#L285
func GetPGXPool(maxConns int) (*pgx.ConnPool, error) {
	creds, err := postgresCredsFromCF()
	if err != nil {
		return nil, err
	}

	return pgx.NewConnPool(pgx.ConnPoolConfig{
		MaxConnections: maxConns,
		ConnConfig: pgx.ConnConfig{
			Database:             creds["name"].(string),
			User:                 creds["username"].(string),
			Password:             creds["password"].(string),
			Host:                 creds["host"].(string),
			Port:                 uint16(creds["port"].(float64)),
			PreferSimpleProtocol: false, // Explicitly declare to force the use of unamed prepared statement which cost 2 trips to db but a lot safer.. Refer to https://gowalker.org/github.com/jackc/pgx#ConnConfig
		},
		// This function is called on every new connection - as explained by jackc/pgx library.
		AfterConnect: (&DBInitter{
			InitSQL: `
				CREATE TABLE IF NOT EXISTS que_jobs (
					priority    smallint    NOT NULL DEFAULT 100,
					run_at      timestamptz NOT NULL DEFAULT now(),
					job_id      bigserial   NOT NULL,
					job_class   text        NOT NULL,
					args        json        NOT NULL DEFAULT '[]'::json,
					error_count integer     NOT NULL DEFAULT 0,
					last_error  text,
					queue       text        NOT NULL DEFAULT '',
					CONSTRAINT que_jobs_pkey PRIMARY KEY (queue, priority, run_at, job_id)
				);

				COMMENT ON TABLE que_jobs IS '3';

				CREATE TABLE IF NOT EXISTS cron_metadata (
					id             text                     PRIMARY KEY,
					last_completed timestamp with time zone NOT NULL DEFAULT TIMESTAMP 'EPOCH',
					next_scheduled timestamp with time zone NOT NULL DEFAULT TIMESTAMP 'EPOCH'
				);

				CREATE TABLE IF NOT EXISTS git_organisations (
					id 					   integer	               NOT NULL,
					name    	           text					   NOT NULL,
					login				   text					   NOT NULL,
					avatar_url			   text					   NOT NULL DEFAULT ''::text,
					raw_description        json                    NOT NULL DEFAULT '[]'::json,
					added_at               timestamptz             NOT NULL DEFAULT now(),
					last_updated           timestamptz             NOT NULL DEFAULT now()
				);
					
				CREATE TABLE IF NOT EXISTS git_repositories (
					id 					   integer	               NOT NULL,
					full_name    	           text					   NOT NULL,
					raw_description        json                    NOT NULL DEFAULT '[]'::json,
					score                  smallint                NOT NULL DEFAULT 100,
					added_at               timestamptz             NOT NULL DEFAULT now(),   
					last_updated           timestamptz             NOT NULL DEFAULT now()
				);

				CREATE TABLE IF NOT EXISTS git_contributors (
					id 			       integer	     NOT NULL,
					login              text          NOT NULL,
					avatar_url         text          NOT NULL DEFAULT ''::text,
					raw_description    json          NOT NULL DEFAULT '[]'::json,
					score              smallint      NOT NULL DEFAULT 100,
					added_at     timestamptz         NOT NULL DEFAULT now(),
					last_updated timestamptz         NOT NULL DEFAULT now()
				);

				CREATE TABLE IF NOT EXISTS git_repository_to_dependency (
					repository_id                       integer    NOT NULL,
					dependency_id            integer    NOT NULL,
					active                      boolean    DEFAULT TRUE,
					added_at     timestamptz    NOT NULL DEFAULT now()
				);

				CREATE TABLE IF NOT EXISTS git_repository_to_contributor (
					repository_id     integer          NOT NULL,
					contributor_id    integer          NOT NULL,
					active            boolean          DEFAULT TRUE,
					added_at        timestamptz        NOT NULL DEFAULT now()
				);

				CREATE TABLE IF NOT EXISTS git_org_to_repository (
					organisation_id                 integer          NOT NULL,
					repository_id          integer          NOT NULL,
					active                 boolean          DEFAULT TRUE,
					added_at     timestamptz   NOT NULL DEFAULT now()
				);

				CREATE TABLE IF NOT EXISTS error_log (
					discovered   timestamptz   NOT NULL DEFAULT now(),
					error        text          NOT NULL
				);
				`,
			OtherStatements:    que.PrepareStatements,
			PreparedStatements: map[string]string{},
		}).AfterConnect,
	})
}
