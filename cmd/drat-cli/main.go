package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/santrancisco/cque"
	"github.com/santrancisco/drat/cmd/drat-cli/jobs"
	"github.com/santrancisco/logutils"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	verbose      = kingpin.Flag("verbose", "Enable debug mode.").Default("false").Short('v').Bool()
	repo         = kingpin.Flag("repo", "Name of repo in the following format: (github.com/(username|org)/repo").Default("").Short('r').String()
	file         = kingpin.Flag("file", "File contains the list of urls to repositories seperated by newlines").Default("").Short('f').String()
	gitauthtoken = kingpin.Flag("gitauth", "Github Authentication token").Default("").OverrideDefaultFromEnvar("GITHUB_AUTH_TOKEN").String()
	outtofile    = kingpin.Flag("output", "If specified, write output to this file").Default("").Short('o').String()
	depth        = kingpin.Flag("depth", "How deep do we want to crawl the dependencies").Default("5").Short('d').Int()
	// sqlitedbpath = kingpin.Flag("sqlpath", "Local sqlite cache location. Default is ~/.drat.sqlite").Default("~/.drat.sqlite").Short('s').String()
)

const (
	ORGTYPE         = "Organisation"
	CONTRIBUTORTYPE = "Contributor"
	REPOSITORYTYPE  = "Repository"
)

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()
	config := map[string]interface{}{}
	config["depth"] = *depth
	config["gitauthtoken"] = *gitauthtoken
	// config["sqlitedbpath"] = *sqlitedbpath

	if (*file == "") && (*repo == "") {
		kingpin.FatalUsage("You need to use this tool against at least one repository")
	}
	// Configuring our log level
	logfilter := "WARNING"
	if *verbose {
		logfilter = "DEBUG"
	}
	filteroutput := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARNING", "INFO", "ERROR"},
		MinLevel: logutils.LogLevel(logfilter),
		Writer:   os.Stderr,
	}
	log.SetOutput(filteroutput)

	ctx, cancel := context.WithCancel(context.Background())
	// defering canclation of all concurence processes
	defer cancel()

	qc := cque.NewQue()
	wpool := cque.NewWorkerPool(qc, cque.WorkMap{
		jobs.KeyScoreGitHubRepo: (&jobs.JobFuncWrapper{
			QC:        qc,
			Logger:    log.New(filteroutput, "", log.LstdFlags),
			F:         jobs.ScoreGitHubRepo,
			AppConfig: config}).Run,

		jobs.KeyListFromFile: (&jobs.JobFuncWrapper{
			QC:        qc,
			Logger:    log.New(filteroutput, "", log.LstdFlags),
			F:         jobs.ListFromFile,
			AppConfig: config}).Run,
	}, 5)

	wpool.Start(ctx)
	// If we give it a file, queue a new job to crawl content of the file.
	if *file != "" {
		qc.Enqueue(cque.Job{Type: jobs.KeyListFromFile, Args: *file})
	}

	if *repo != "" {
		qc.Enqueue(cque.Job{
			Type: jobs.KeyScoreGitHubRepo,
			Args: jobs.RepoJob{Fullname: *repo, DependedOnBy: "", Currentdepth: 0},
		})
	}

	time.Sleep(2 * time.Second)
	rh := ResultHandler{
		WaitForResult: true,
		WaitTimeStart: time.Now(),
		qc:            qc,
		ctx:           ctx,
	}
	go rh.Run()
	running := true
	for running {
		// If we have been waiting for result for more than 1s.
		if rh.WaitForResult && (time.Since(rh.WaitTimeStart) > (time.Duration(2) * time.Second)) {
			shouldnotrunning := qc.IsQueueEmpty && rh.WaitForResult
			running = !shouldnotrunning
		} else {
			running = true
		}
		// when queue is not empty we loop mainthread.
		time.Sleep(1 * time.Second)
	}
	b, err := json.MarshalIndent(rh.Results, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if *outtofile != "" {
		// delete file
		_ = os.Remove(*outtofile)
		log.Printf("[DEBUG] Done deleting file %s", *outtofile)
		f, err := os.OpenFile(*outtofile, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			fmt.Println(string(b))
			fmt.Printf("Having issue with opening file %s. The result is printed to stdout \n", *outtofile)
			log.Fatal(err)
		}
		f.Write(b)
		log.Printf("[DEBUG] Done writting a new ouput file %s", *outtofile)
		log.Printf("[INFO] You can use ./static/index.html in chrome to parse the output.")
	} else {
		// If we don't specify a file output, dump it to stdout
		fmt.Println(string(b))
	}

}

// TODO: Find home for result handler. :)
type Result interface {
	String() string
}

type ResultHandler struct {
	// Interval is the amount of time that this Worker should sleep before trying
	// to find another Job.
	WaitForResult bool
	WaitTimeStart time.Time
	Results       []Result
	qc            *cque.Client
	ctx           context.Context
}

func (rh *ResultHandler) Run() {
	for {
		rh.WaitForResult = true
		rh.WaitTimeStart = time.Now()
		select {
		case <-rh.ctx.Done():
			log.Printf("[DEBUG] Result Handler is done\n")
			return
		case r := <-rh.qc.Result:
			result := r.Result.(jobs.ScoreGitHubRepoFuncResult)
			rh.Results = append(rh.Results, &result)
		}
	}
}
