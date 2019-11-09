package crawl

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/go-github/github"
)

type Dependencyproblem struct {
	Name      string
	URL       string
	RiskNotes []string
}

// "https://raw.githubusercontent.com/%s/%s/master/%s",owner,repository,filename

func GithubDependencyCrawl(logger *log.Logger, c *github.Client, r *github.Repository, config interface{}) ([]string, []Dependencyproblem) {
	supportfiles := make(map[string]ParserFunc)
	supportfiles["Gopkg.lock"] = ParseGoDep
	supportfiles["package.json"] = ParseNPM
	supportfiles["Gemfile"] = ParseGem
	supportfiles["requirements.txt"] = ParsePip
	supportfiles["go.mod"] = ParseGoMod

	var listOfDependencies []string
	var listOfProblems []Dependencyproblem
	for filename, PF := range supportfiles {
		dependencies, problems, err := GetFileAndParseResult(filename, PF, logger, c, r, config)
		if err != nil {
			continue
		}
		if len(dependencies) > 0 {
			listOfDependencies = append(listOfDependencies, dependencies...)
		}
		if len(problems) > 0 {
			listOfProblems = append(listOfProblems, problems...)
		}
	}
	return listOfDependencies, listOfProblems
}

type ParserFunc func(filecontent []byte) ([]string, []Dependencyproblem)

func GetFileAndParseResult(filename string, PF ParserFunc, logger *log.Logger, c *github.Client, r *github.Repository, config interface{}) ([]string, []Dependencyproblem, error) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/%s", *r.Owner.Login, *r.Name, filename))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == 200 {
		logger.Print(fmt.Sprintf("[DEBUG] Successfully downloaded https://raw.githubusercontent.com/%s/%s/master/%s", *r.Owner.Login, *r.Name, filename))
		contentbytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		d, p := PF(contentbytes)
		return d, p, nil
	}
	return nil, nil, nil
}
