package people

import (
	"encoding/json"
	"fmt"
	"net/http"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
	"net/url"
	"strings"
)

// PeopleDriver for cypher queries
var PeopleDriver Driver
var CacheControlHeader string

//var maxAge = 24 * time.Hour

// HealthCheck does something
func HealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact: "Unable to respond to Public People api requests",
		Name:           "Check connectivity to Neo4j - neoUrl is a parameter in hieradata for this service",
		PanicGuide:     "https://sites.google.com/a/ft.com/ft-technology-service-transition/home/run-book-library/public-people-api",
		Severity:       1,
		TechnicalSummary: `Cannot connect to Neo4j. If this check fails, check that Neo4j instance is up and running. You can find
				the neoUrl as a parameter in hieradata for this service. `,
		Checker: Checker,
	}
}

// Checker does more stuff
func Checker() (string, error) {
	err := PeopleDriver.CheckConnectivity()
	if err == nil {
		return "Connectivity to neo4j is ok", err
	}
	return "Error connecting to neo4j", err
}

// Ping says pong
func Ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "pong")
}

//GoodToGo returns a 503 if the healthcheck fails - suitable for use from varnish to check availability of a node
func GoodToGo(writer http.ResponseWriter, req *http.Request) {
	if _, err := Checker(); err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}

}

// BuildInfoHandler - This is a stop gap and will be added to when we can define what we should display here
func BuildInfoHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "build-info")
}

// MethodNotAllowedHandler handles 405
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	return
}

// GetPerson is the public API
func GetPerson(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestedId, err := uuid.FromString(vars["uuid"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	person, found, err := PeopleDriver.Read(requestedId.String())
	if err != nil {
		log.WithFields(log.Fields{"requestedId": requestedId, "err": err}).Debug("Redirecting...")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "` + err.Error() + `"}`))
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Person not found."}`))
		return
	}

	// if the client requested the non-canonical UUID, then we redirect them to the URL for the canonical ID.
	canonicalId, err := extractCanonicalUUID(person)
	if err != nil {
		log.WithFields(log.Fields{"ID": person.ID, "err": err}).Error("Error reading canonical ID")
	}
	if !uuid.Equal(canonicalId, requestedId) {
		log.WithFields(log.Fields{"canonicalId": canonicalId, "requestedId": requestedId}).Debug("Redirecting...")
		redirectURL := strings.Replace(r.RequestURI, requestedId.String(), canonicalId.String(), 1)
		w.Header().Set("Location", redirectURL)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	w.Header().Set("Cache-Control", CacheControlHeader)
	w.WriteHeader(http.StatusOK)

	if err = json.NewEncoder(w).Encode(person); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Person could not be retrieved, err=` + err.Error() + `"}`))
	}
}

// extract the UUID from the person ID URL by taking the last element of the path.
func extractCanonicalUUID(person Person) (uuid.UUID, error) {
	u, err := url.Parse(person.ID)
	path := strings.Split(u.Path, "/")
	id, err := uuid.FromString(path[len(path)-1])
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
