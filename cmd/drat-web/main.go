package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bgentry/que-go"

	"github.com/santrancisco/drat/cmd/drat-web/db"
	"github.com/santrancisco/drat/cmd/drat-web/jobs"
)

// Declare how many threads you want per instance.
const (
	WorkerCount = 5
)

func main() {
	pgxPool, err := db.GetPGXPool(WorkerCount * 2)
	if err != nil {
		log.Fatal(err)
	}

	qc := que.NewClient(pgxPool)
	// Foreach worker, we register the WorkMap ("Job_name_string_in_database" : Function)
	workers := que.NewWorkerPool(qc, que.WorkMap{
		jobs.KeyGitListRepoFromOrg: (&jobs.JobFuncWrapper{ // JobFunc to get list of repositories from an Organisation
			QC:        qc,
			Logger:    log.New(os.Stderr, jobs.KeyGitListRepoFromOrg+" ", log.LstdFlags),
			F:         jobs.GitListRepoFromOrg,
			Singleton: true,           // This job can only have 1 instance at a time
			Duration:  time.Hour * 24, // This job run once a day
		}).Run,
		jobs.KeyGitScoreRepository: (&jobs.JobFuncWrapper{ // Jobfunc to score repository
			QC:     qc,
			Logger: log.New(os.Stderr, jobs.KeyGitScoreRepository+" ", log.LstdFlags),
			F:      jobs.GitScoreRepository,
		}).Run,
		jobs.KeyGitScoreContributor: (&jobs.JobFuncWrapper{ // Jobfunc to score contributor
			QC:     qc,
			Logger: log.New(os.Stderr, jobs.KeyGitScoreContributor+" ", log.LstdFlags),
			F:      jobs.GitScoreContributor,
		}).Run,
		jobs.KeyGitGetDependenciesGo: (&jobs.JobFuncWrapper{ // Jobfunc support dependencies crawling for go projects
			QC:     qc,
			Logger: log.New(os.Stderr, jobs.KeyGitGetDependenciesGo+" ", log.LstdFlags),
			F:      jobs.GitGetDependenciesGo,
		}).Run,
		jobs.KeyGitListContributorFromRepo: (&jobs.JobFuncWrapper{ // Jobfunc to retrieve the list of distributors of a repository.
			QC:     qc,
			Logger: log.New(os.Stderr, jobs.KeyGitListContributorFromRepo+" ", log.LstdFlags),
			F:      jobs.GitListContributorFromRepo,
		}).Run,
	}, WorkerCount)

	// Prepare a shutdown function
	shutdown := func() {
		workers.Shutdown()
		pgxPool.Close()
	}

	// Normal exit
	defer shutdown()

	// Or via signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received %v, starting shutdown...", sig)
		shutdown()
		log.Println("Shutdown complete")
		os.Exit(0)
	}()

	go workers.Start()

	// Seed it with our first job - Crawl AusDTO organisation.
	err = qc.Enqueue(&que.Job{
		Type:  jobs.KeyGitListRepoFromOrg,
		Args:  []byte("{\"Name\":\"AusDTO\"}"),
		RunAt: time.Now(),
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Started up... waiting for ctrl-C.")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Up and away.")
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil))
}
