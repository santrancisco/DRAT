package score

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/santrancisco/drat/crawl"
)

func TestParseGoDep(t *testing.T) {
	t.Log("Test ParseGoDep")
	content, err := ioutil.ReadFile("Gopkg.lock")
	if err != nil {
		t.Errorf("Cannot open Gopkg.lock")
	}
	lst, _ := crawl.ParseGoDep(content)
	final := strings.Join(lst, ",")
	expected := "github.com/beorn7/perks,github.com/bgentry/que-go,github.com/cloudfoundry-community/go-cfenv,github.com/gogo/protobuf,github.com/golang/protobuf,github.com/google/certificate-transparency-go,github.com/jackc/pgx,github.com/matttproud/golang_protobuf_extensions,github.com/mitchellh/mapstructure,github.com/pkg/errors,github.com/prometheus/client_golang,github.com/prometheus/client_model,github.com/prometheus/common,github.com/prometheus/procfs,golang.org/x/crypto,golang.org/x/net"
	if final != expected {
		t.Errorf("ParseGoDep fails, output is not as expected")
	}
}
