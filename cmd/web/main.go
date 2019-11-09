package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/abbot/go-http-auth"
	cfenv "github.com/cloudfoundry-community/go-cfenv"
	github "github.com/google/go-github/github"
	"github.com/jackc/pgx"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	debugflag     = kingpin.Flag("debug", "Enable debug mode.").Default("false").Short('d').Bool()
	httpport      = kingpin.Flag("port", "create a HTTP listener to satisfy CF healthcheck requirement").Default("8080").OverrideDefaultFromEnvar("VCAP_APP_PORT").Short('p').String()
	staticpath    = kingpin.Flag("static", "Static folder location").Default("./static").OverrideDefaultFromEnvar("STATIC_PATH").Short('s').String()
	httponly      = kingpin.Flag("HTTPOnly", "Skip checking with Github API to test HTTP server").Default("false").Short('O').Bool()
	basicpassword = kingpin.Flag("basicpass", "Change basicauth password").Default("$1$dlPL2MqE$oQmn16q49SqdmhenQuNgs1").OverrideDefaultFromEnvar("BASICPASS").Short('b').String()
)

const (
	ORGTYPE         = "Organisation"
	CONTRIBUTORTYPE = "Contributor"
	REPOSITORYTYPE  = "Repository"
)

type Element struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Depends      []string `json:"depends"`
	DependedOnBy []string `json:"dependedOnBy"`
	DocURL       string   `json:"docurl"`
	Docs         string   `json:"docs"`
}

type GraphData struct {
	Data   map[string]*Element `json:"data"`
	Errors []string            `json:"errors"`
}

type server struct {
	DB *pgx.ConnPool
}

// Secret function is function provide simple basic authentication for  "github.com/abbot/go-http-auth"
func Secret(user, realm string) string {
	if user == "secop" {
		// dev default password is `hello` - this will be replaced with ENV in prod
		return *basicpassword
	}
	return ""
}

func postgresCredsFromCF() (map[string]interface{}, error) {
	appEnv, err := cfenv.Current()
	if err != nil {
		return nil, err
	}

	dbEnv, err := appEnv.Services.WithTag("postgres")
	if err != nil {
		return nil, err
	}

	if len(dbEnv) != 1 {
		return nil, errors.New("expecting 1 database")
	}

	return dbEnv[0].Credentials, nil
}

func (s *server) GetOrgRelation(key string, id int, g *GraphData) error {
	// Get Organisation to Repository relationship
	rows, err := s.DB.Query("SELECT repository_id,active, added_at FROM git_org_to_repository where organisation_id = $1", id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var repository_id int
		var active bool
		var added_at time.Time
		err = rows.Scan(&repository_id, &active, &added_at)
		if err != nil {
			return err
		}
		g.Data[key].Depends = append(g.Data[key].Depends, fmt.Sprintf("r-%v", repository_id))
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}
func (s *server) GetRepoRelation(key string, id int, g *GraphData) error {
	// Get Dependencies relationships
	rows, err := s.DB.Query("SELECT dependency_id,active, added_at FROM git_repository_to_dependency where repository_id = $1", id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var dependency_id int
		var active bool
		var added_at time.Time
		err = rows.Scan(&dependency_id, &active, &added_at)
		if err != nil {
			return err
		}
		g.Data[key].Depends = append(g.Data[key].Depends, fmt.Sprintf("r-%v", dependency_id))
	}
	if rows.Err() != nil {
		return err
	}

	// Get Contributor to Repository relationships
	rows, err = s.DB.Query("SELECT contributor_id,active, added_at FROM git_repository_to_contributor where repository_id = $1", id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var contributor_id int
		var active bool
		var added_at time.Time
		err = rows.Scan(&contributor_id, &active, &added_at)
		if err != nil {
			return err
		}
		g.Data[key].Depends = append(g.Data[key].Depends, fmt.Sprintf("c-%v", contributor_id))
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

func (s *server) GetRelation(g *GraphData) error {
	for key, value := range g.Data {
		switch value.Type {
		case ORGTYPE:
			s.GetOrgRelation(key, value.ID, g)
		case REPOSITORYTYPE:
			s.GetRepoRelation(key, value.ID, g)
		default:
			continue
		}
	}
	return nil
}

func (s *server) GetOrgs(g *GraphData) error {
	rows, err := s.DB.Query("SELECT id, name, raw_description FROM git_organisations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		var raw_description []byte
		err = rows.Scan(&id, &name, &raw_description)
		if err != nil {
			return err
		}
		var o github.Organization
		err = json.Unmarshal(raw_description, &o)
		if err != nil {
			return err
		}
		g.Data[fmt.Sprintf("o-%v", id)] = &Element{ID: id, Name: name, Type: ORGTYPE, DocURL: o.GetHTMLURL(), Docs: string(raw_description), Depends: []string{}, DependedOnBy: []string{}}
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

func (s *server) GetRepositories(g *GraphData) error {
	rows, err := s.DB.Query("SELECT id, full_name, raw_description FROM git_repositories")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		var raw_description []byte
		err = rows.Scan(&id, &name, &raw_description)
		if err != nil {
			return err
		}
		var r github.Repository
		err = json.Unmarshal(raw_description, &r)
		if err != nil {
			return err
		}
		g.Data[fmt.Sprintf("r-%v", id)] = &Element{ID: id, Name: name, Type: REPOSITORYTYPE, DocURL: r.GetHTMLURL(), Docs: string(raw_description), Depends: []string{}, DependedOnBy: []string{}}
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

func (s *server) GetContributors(g *GraphData) error {
	rows, err := s.DB.Query("SELECT id, login, raw_description FROM git_contributors")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		var raw_description []byte
		err = rows.Scan(&id, &name, &raw_description)
		if err != nil {
			return err
		}
		var c github.Contributor
		err = json.Unmarshal(raw_description, &c)
		if err != nil {
			return err
		}
		g.Data[fmt.Sprintf("c-%v", id)] = &Element{ID: id, Name: name, Type: CONTRIBUTORTYPE, DocURL: c.GetHTMLURL(), Docs: string(raw_description), Depends: []string{}, DependedOnBy: []string{}}
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

func GetPGXPool(maxConns int) (*pgx.ConnPool, error) {
	creds, err := postgresCredsFromCF()
	if err != nil {
		return nil, err
	}

	return pgx.NewConnPool(pgx.ConnPoolConfig{
		MaxConnections: maxConns,
		ConnConfig: pgx.ConnConfig{
			Database:             creds["name"].(string),
			User:                 creds["username"].(string),
			Password:             creds["password"].(string),
			Host:                 creds["host"].(string),
			Port:                 uint16(creds["port"].(float64)),
			PreferSimpleProtocol: false, // Explicitly declare to force the use of unamed prepared statement which cost 2 trips to db but a lot safer.. Refer to https://gowalker.org/github.com/jackc/pgx#ConnConfig
		}})
}

func (s *server) getGraphData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	//w.Write()

	var g GraphData
	g.Errors = []string{}
	g.Data = make(map[string]*Element)

	s.GetOrgs(&g)
	s.GetRepositories(&g)
	s.GetContributors(&g)
	s.GetRelation(&g)

	graphdata, err := json.Marshal(g)
	if err != nil {
		w.Write([]byte("Error"))
	}

	w.Write(graphdata)
}

// func (s *server) demogetGraphData(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	//w.Write()

// 	var meh GraphData
// 	meh.Data = make(map[string]Element)
// 	meh.Data["repo"] = Element{"repo", "group1", []string{"org"}, []string{}, "url", "Example 1"}
// 	meh.Data["repo1"] = Element{"repoa", "group1", []string{"org"}, []string{}, "url", "Example 1"}
// 	meh.Data["repo2"] = Element{"repob", "group1", []string{"org"}, []string{}, "url", "Example 1"}
// 	meh.Data["repo3"] = Element{"repoc", "group1", []string{"org"}, []string{}, "url", "Example 1"}
// 	meh.Data["org"] = Element{"AusDTA", "group0", []string{}, []string{}, "url", "Example 2"}
// 	meh.Data["contrib"] = Element{"contrib", "group3", []string{"repo"}, []string{}, "url", "Example 3"}
// 	meh.Errors = []string{}
// 	graphdata, err := json.Marshal(meh)
// 	if err != nil {
// 		w.Write([]byte("Error"))
// 	}
// 	w.Write(graphdata)
// }

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()

	pg, err := GetPGXPool(5)
	if err != nil {
		log.Fatal(err)
	}
	defer pg.Close()

	s := &server{
		DB: pg,
	}
	_ = s

	authenticator := auth.NewBasicAuthenticator("secop", Secret)
	fs := http.FileServer(http.Dir(*staticpath))
	http.Handle("/", authenticator.Wrap(func(res http.ResponseWriter, req *auth.AuthenticatedRequest) {
		fs.ServeHTTP(res, &req.Request)
	}))
	// Updating pentest status
	http.Handle("/data", authenticator.Wrap(func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		if r.Method == "GET" {
			s.getGraphData(w, &r.Request)
		} else {
			http.Error(w, "Invalid request method.", 405)
		}
	}))

	log.Println("HTTP port is litening...")
	http.ListenAndServe(fmt.Sprintf(":%s", *httpport), nil)
	panic("HTTP server failed to listen")

}
