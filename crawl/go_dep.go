package crawl

import (
	"regexp"
)

func ParseGoDep(contentbytes []byte) ([]string, []Dependencyproblem) {
	gopkgfile := string(contentbytes)
	// Matching something like `name = "github.com/beorn7/perks"`
	rex := regexp.MustCompile(`name = \"([^\"]+/[^\"]+/[^\"]+)\"`)
	out := rex.FindAllStringSubmatch(gopkgfile, -1)
	//fmt.Println(out)
	var retlst []string
	for _, i := range out {
		// construct our return list
		retlst = append(retlst, i[1])
	}
	return retlst, nil
}
