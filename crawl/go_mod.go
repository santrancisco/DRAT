package crawl

import (
	"regexp"
)

func ParseGoMod(contentbytes []byte) ([]string, []Dependencyproblem) {
	gomodfile := string(contentbytes)
	// Matching something like `name = "github.com/beorn7/perks"`
	rex := regexp.MustCompile(`\t([^ ]+\/[^ ]+\/[^ ]+)`)
	out := rex.FindAllStringSubmatch(gomodfile, -1)
	var retlst []string
	for _, i := range out {
		// construct our return list
		retlst = append(retlst, i[1])
	}
	return retlst, nil
}
