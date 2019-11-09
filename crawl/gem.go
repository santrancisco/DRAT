package crawl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

const (
	RubygemURLprefix = "https://rubygems.org/api/v1/gems"
)

type geminfo struct {
	Source_code_uri string `json:"source_code_uri"`
}

func ParseGem(contentbytes []byte) ([]string, []Dependencyproblem) {
	var retlst []string
	content := string(contentbytes)
	rex := regexp.MustCompile(`gem \'([^\']+)\'`)
	listofgem := rex.FindAllStringSubmatch(content, -1)

	for _, r := range listofgem {
		resp, err := http.Get(fmt.Sprintf("%s/%s.json", RubygemURLprefix, r[1]))
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

		var g geminfo
		err = json.Unmarshal(b, &g)
		if err != nil {
			log.Print(err.Error())
			continue
		}
		if len(g.Source_code_uri) > 5 {
			retlst = append(retlst, g.Source_code_uri)
		}

	}
	return retlst, nil
}
