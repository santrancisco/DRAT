package crawl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/anaskhan96/soup"
)

const (
	PypiURLprefix = "https://pypi.org/project"
	//Libraries.IO is a great tool but rate limiting is cap at 60 requests per minute so unless
	// there is a way to up the limit, we cant use this for now.
	LibrariesIOprefix = "https://libraries.io/api/pypi"
)

type librariesio struct {
	Repository_url string `json:"repository_url"`
}

func TryLibrariesIO(name string) string {
	log.Printf("[DEBUG] Trying to acquire repository url for %s from libraries.io", name)
	resp, err := http.Get(fmt.Sprintf("%s/%s", LibrariesIOprefix, name))
	if err != nil {
		log.Printf("[ERROR] FAILED to download")
		return ""
	}
	if resp.StatusCode != 200 {
		log.Printf("[ERROR] Response was no 200")
		return ""
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var l librariesio
	err = json.Unmarshal(b, &l)
	if err != nil {
		log.Printf("[ERROR] Marshalling problem")
		return ""
	}
	return l.Repository_url
}

func ParsePip(contentbytes []byte) ([]string, []Dependencyproblem) {
	var retlst []string
	var problems = []Dependencyproblem{}
	content := string(contentbytes)
	rexgit := regexp.MustCompile(`git\+(.*)\.git`)
	rexlib := regexp.MustCompile(`(?m:^)[\s+]?([a-zA-Z0-9]+[a-zA-Z0-9-_.]+( +)?)(([><=]=)|(?m:$))`)
	listofgit := rexgit.FindAllStringSubmatch(content, -1)
	listoflib := rexlib.FindAllStringSubmatch(content, -1)
	if len(listofgit) > 0 {
		for _, v := range listofgit {
			retlst = append(retlst, v[1])
		}
	}
	for _, r := range listoflib {
		libname := r[1]
		resp, err := http.Get(fmt.Sprintf("%s/%s", PypiURLprefix, libname))
		if err != nil {
			continue
		}
		if resp.StatusCode != 200 {
			problems = append(problems, Dependencyproblem{
				Name:      libname,
				URL:       fmt.Sprintf("%s/%s", PypiURLprefix, libname),
				RiskNotes: []string{"[MEDIUM] Could not retrieve information from pypi website for this library"},
			})
			continue
		}
		dp := Dependencyproblem{
			Name:      libname,
			URL:       fmt.Sprintf("%s/%s", PypiURLprefix, libname),
			RiskNotes: []string{},
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		doc := soup.HTMLParse(string(b))
		p := doc.Find("div", "id", "description").FindAll("p")
		if len(p) > 0 {
			if strings.TrimSpace(p[0].Text()) == "The author of this package has not provided a project description" {
				dp.RiskNotes = append(dp.RiskNotes, "[MEDIUM] This project does not have a description page on Pypi")
			}
		}

		// Lets try to find a github,bitbucket and gitlab project URL
		rexgh := regexp.MustCompile(fmt.Sprintf("(((github.com)|(bitbucket.org)|(gitlab.com))/(repos/)?[a-zA-Z0-9-_]+/((%s)|(%s)))", libname, strings.ToLower(libname)))
		ghlinks := rexgh.FindAllStringSubmatch(string(b), -1)
		if len(ghlinks) == 0 {
			// Because LibrariesIO has strict ratelimit, we are using it as second attempt to get repository url
			rurl := TryLibrariesIO(libname)
			if rurl != "" && rurl != "null" {
				log.Printf("[INFO] Found repourl %s using libraries.io", rurl)
				retlst = append(retlst, rurl)
				// If this still fails, we are reporting it as an issue
			} else {
				dp.RiskNotes = append(dp.RiskNotes, "[MEDIUM] Pypi page does not have any reference to a repository")
				sidebars := doc.FindAll("div", "class", "sidebar-section")
				// We check if the project at least include a homepage for the project (in many cases, developers are lazy and only include the homepage url)
				for _, v := range sidebars {
					links := v.FindAll("a")
					found := false
					if len(links) > 0 {
						for _, link := range links {
							if strings.ToLower(strings.TrimSpace(link.Text())) == "homepage" {
								found = true
								dp.RiskNotes = append(dp.RiskNotes, fmt.Sprintf("[INFO] Pypi does has homepage for project at %s", link.Attrs()["href"]))
								break
							}
						}
					}
					if found == true {
						break
					}
				}
			}
		} else {
			retlst = append(retlst, ghlinks[0][1])
		}
		// If we have a risknote related to Pypi, add it to the list of dependencies problems
		if len(dp.RiskNotes) > 0 {
			problems = append(problems, dp)
		}

	}
	return retlst, problems
}
