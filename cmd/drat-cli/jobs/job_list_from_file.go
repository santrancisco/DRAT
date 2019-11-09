package jobs

import (
	"log"
	"strings"

	crawl "github.com/santrancisco/drat/crawl"
	"github.com/santrancisco/cque"
)

const (
	KeyListFromFile = "list_from_file"
)

func ListFromFile(logger *log.Logger, qc *cque.Client, j *cque.Job, config map[string]interface{}) error {
	filename := j.Args.(string)
	lst, err := crawl.ListFromFile(filename)
	if err != nil {
		return err
	}
	for _, v := range lst {
		if strings.Contains(v, "github.com") {
			qc.Enqueue(cque.Job{
				Type: KeyScoreGitHubRepo,
				Args: RepoJob{Fullname: v, DependedOnBy: "", Currentdepth: 0},
			})
			continue
		}
		// If it does not match with any type of Scoring job, discard it
		logger.Printf("[INFO] No scoring job for \"%s\" in file %s\n", v, filename)
	}

	return nil
}
