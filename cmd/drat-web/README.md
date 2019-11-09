### Intro  

Development for this app is currently on HOLD while working on the DRAT-cli tool.

This app use postgresql and focus on running DRAT as a service/web application. 

### Dependency Risk Analysis Tool - DRAT 

DRAT aims to provide risk indicator for libraries used by the developer within an organisation.

This tool's intial code and design is inspired by the certificate transparency work done by Adam Eijdenberg from Cloud.gov.au team. You can find the repository for that [here](https://github.com/govau/certwatch/tree/master/jobs)

## Running locally

```bash
docker run -p 5432:5432 --name dratpg -e POSTGRES_USER=dratpg -e POSTGRES_PASSWORD=dratpg -d postgres
export VCAP_SERVICES='{"postgres": [{"credentials": {"username": "certpg", "host": "localhost", "password": "certpg", "name": "certpg", "port": 5434}, "tags": ["postgres"]}]}'
go run *.go
```

## Running in docker with latest code

```bash
GOOS=linux GOARCH=amd64 go build -o bin/drat cmd/drat/main.go
docker-compose up
```

To checkout database:

```bash
psql "dbname=dratpg host=localhost user=dratpg password=dratpg port=5432"
```

