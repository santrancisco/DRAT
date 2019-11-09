package score

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/santrancisco/drat/crawl"
)

// Note: These are not proper unit test, these are being used to test run the function without having to run the whole app at the moment. These will
// be converted into proper test in the future.
func TestParseNPM(t *testing.T) {
	t.Log("Test Parse")
	content, err := ioutil.ReadFile("package.json")
	if err != nil {
		t.Errorf("Cannot open package.json")
	}
	lst, _ := crawl.ParseNPM(content)
	final := strings.Join(lst, ",")
	fmt.Println(final)
	// expected := "github.com/beorn7/perks,github.com/bgentry/que-go,github.com/cloudfoundry-community/go-cfenv,github.com/gogo/protobuf,github.com/golang/protobuf,github.com/google/certificate-transparency-go,github.com/jackc/pgx,github.com/matttproud/golang_protobuf_extensions,github.com/mitchellh/mapstructure,github.com/pkg/errors,github.com/prometheus/client_golang,github.com/prometheus/client_model,github.com/prometheus/common,github.com/prometheus/procfs,golang.org/x/crypto,golang.org/x/net"
	// if final != expected {
	// 	t.Errorf("ParseGoDep fails, output is not as expected")
	// }
}
