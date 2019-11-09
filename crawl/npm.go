package crawl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type packagejsonfile struct {
	Name         string            `json:"name"`
	Dependencies map[string]string `json:"dependencies"`
}

const (
	NPMregistryURL = "https://registry.npmjs.org"
)

type npminfo struct {
	Repository struct {
		Type string `json:"type"`
		Url  string `json:"url"`
	} `json:"repository"`
}

func ParseNPM(contentbytes []byte) ([]string, []Dependencyproblem) {
	var retlst []string
	var pjfile packagejsonfile
	err := json.Unmarshal(contentbytes, &pjfile)
	if err != nil {
		// Return empty list if we get an error here
		log.Printf("[ERROR] Could not parse content of a package.json file")
		return nil, nil
	}
	for name, _ := range pjfile.Dependencies {
		resp, err := http.Get(fmt.Sprintf("%s/%s/", NPMregistryURL, name))
		if err != nil {
			continue
		}
		if resp.StatusCode != 200 {
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		var n npminfo
		err = json.Unmarshal(b, &n)
		if err != nil {
			fmt.Print(err.Error())
			continue
		}
		if strings.ToLower(n.Repository.Type) == "git" {
			url := strings.TrimPrefix(n.Repository.Url, "git+")
			url = strings.TrimSuffix(url, ".git")
			retlst = append(retlst, url)
		}

	}
	return retlst, nil
}
