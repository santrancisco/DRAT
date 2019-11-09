package crawl

import "io/ioutil"
import "strings"

func ListFromFile(filename string) ([]string, error) {
	contentbyte, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// Trimming all space, newline characters from the content
	content := strings.Trim(string(contentbyte), " \t\r\n")
	// Spliting all new lines
	lst := strings.Split(string(content), "\n")
	// We are trimming all values too for a clean list
	for i, v := range lst {
		lst[i] = strings.Trim(v, " \t\r")
	}
	return lst, nil
}
