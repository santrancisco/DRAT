package score

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/santrancisco/drat/crawl"
)

func TestParseGoMod(t *testing.T) {
	t.Log("Test ParseGoMod")
	content, err := ioutil.ReadFile("go.mod")
	if err != nil {
		t.Errorf("Cannot open go.mod")
	}
	lst, _ := crawl.ParseGoMod(content)
	final := strings.Join(lst, ",")
	expected := "github.com/santrancisco/drat,github.com/abbot/go-http-auth,github.com/alecthomas/template,github.com/alecthomas/units,github.com/anaskhan96/soup"
	if final != expected {
		t.Errorf("ParseGoMod fails, output is not as expected")
	}
}
