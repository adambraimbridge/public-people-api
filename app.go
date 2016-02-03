package main

import (
	"net/http"
	"os"

	"github.com/Financial-Times/base-ft-rw-app-go"
	"github.com/Financial-Times/go-fthealth/v1a"
	"github.com/Financial-Times/http-handlers-go"
	"github.com/Financial-Times/public-people-api/people"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"github.com/jmcvetta/neoism"
	"github.com/rcrowley/go-metrics"
)

func main() {
	log.Infof("Application starting with args %s", os.Args)
	app := cli.App("public-people-api-neo4j", "A public RESTful API for accessing People in neo4j")
	neoURL := app.StringOpt("neo-url", "http://localhost:7474/db/data", "neo4j endpoint URL")
	//neoURL := app.StringOpt("neo-url", "http://ftper58827-law1b-eu-t:8080/db/data", "neo4j endpoint URL")
	port := app.StringOpt("port", "8080", "Port to listen on")
	env := app.StringOpt("env", "local", "environment this app is running in")
	graphiteTCPAddress := app.StringOpt("graphiteTCPAddress", "",
		"Graphite TCP address, e.g. graphite.ft.com:2003. Leave as default if you do NOT want to output to graphite (e.g. if running locally)")
	graphitePrefix := app.StringOpt("graphitePrefix", "",
		"Prefix to use. Should start with content, include the environment, and the host name. e.g. content.test.public.people.api.ftaps59382-law1a-eu-t")
	logMetrics := app.BoolOpt("logMetrics", false, "Whether to log metrics. Set to true if running locally and you want metrics output")

	app.Action = func() {
		baseftrwapp.OutputMetricsIfRequired(*graphiteTCPAddress, *graphitePrefix, *logMetrics)

		if *env != "local" {
			f, err := os.OpenFile("/var/log/apps/public-people-api-go-app.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
			if err == nil {
				log.SetOutput(f)
			} else {
				log.Fatalf("Failed to initialise log file, %v", err)
			}

			defer f.Close()
		}

		log.Infof("public-people-api will listen on port: %s, connecting to: %s", *port, *neoURL)
		runServer(*neoURL, *port)
	}

	log.SetLevel(log.InfoLevel)
	log.Infof("Application started with args %s", os.Args)
	app.Run(os.Args)
}

func runServer(neoURL string, port string) {
	db, err := neoism.Connect(neoURL)
	db.Session.Client = &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 100}}
	if err != nil {
		log.Fatalf("Error connecting to neo4j %s", err)
	}
	people.PeopleDriver = people.NewCypherDriver(db)

	router := mux.NewRouter()

	// Healthchecks and standards first
	router.HandleFunc("/__health", v1a.Handler("PeopleReadWriteNeo4j Healthchecks",
		"Checks for accessing neo4j", people.HealthCheck()))
	router.HandleFunc("/ping", people.Ping)
	router.HandleFunc("/__ping", people.Ping)

	// The top one of these feels more correct, but the lower one matches what we have in Dropwizard,
	// so it's what apps expect currently same as ping, the content of build-info needs more definition
	router.HandleFunc("/__build-info", people.BuildInfoHandler)
	router.HandleFunc("/build-info", people.BuildInfoHandler)

	// Then API specific ones:
	router.HandleFunc("/people/{uuid}", people.GetPerson).Methods("GET")

	var handler http.Handler = router
	handler = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), handler)
	handler = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, handler)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Unable to start server: %v", err)
	}
}
