package jobs

import (
	"log"
	"os"
	"strings"
	"time"

	que "github.com/bgentry/que-go"
	"github.com/jackc/pgx"
)

const (
	KeyGitGetDependenciesGo = "git_get_dependencies_go"
)

// Return an array of string from comma seperated environment variable
func listFromEnvVariable(key string) []string {
	e, exist := os.LookupEnv(key)
	if !exist {
		return nil
	}
	return strings.Split(e, ",")
}

//GetDependenciesGo : A jobfunc to retrieve list of dependencies and sort out ones on github so we can queue more jobs for rating
func GitGetDependenciesGo(qc *que.Client, logger *log.Logger, job *que.Job, tx *pgx.Tx) error {
	// Simulating this jobfunc for testing purpose
	time.Sleep(2 * time.Second)

	return nil
}
