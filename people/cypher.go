package people

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jmcvetta/neoism"
)

// Driver interface
type Driver interface {
	Read(id string) (person Person, found bool, err error)
	CheckConnectivity() (ok bool, err error)
}

// CypherDriver struct
type CypherDriver struct {
	db *neoism.Database
}

//NewCypherDriver instanciate driver
func NewCypherDriver(db *neoism.Database) CypherDriver {
	return CypherDriver{db}
}

// CheckConnectivity tests neo4j by running a simple cypher query
func (pcw CypherDriver) CheckConnectivity() (bool, error) {
	results := []struct {
		ID int
	}{}
	query := &neoism.CypherQuery{
		Statement: "MATCH (p:Person) RETURN p.id LIMIT 1",
		Result:    &results,
	}
	err := pcw.db.Cypher(query)
	ok := len(results) == 1 && results[0].ID != 0
	log.Debugf("CheckConnectivity %t %+v", ok, err)
	return ok, err
}

type neoChangeEvent struct {
	StartedAt string
	EndedAt   string
}

type neoReadStruct struct {
	P struct {
		ID        string
		Types     []string
		PrefLabel string
		Labels    []string
	}
	M []struct {
		M struct {
			ID           string
			Types        []string
			PrefLabel    string
			Title        string
			ChangeEvents []neoChangeEvent
		}
		O struct {
			ID        string
			Types     []string
			PrefLabel string
			Labels    []string
		}
		R []struct {
			ID           string
			Types        []string
			PrefLabel    string
			ChangeEvents []neoChangeEvent
		}
	}
}

func (pcw CypherDriver) Read(uuid string) (person Person, found bool, err error) {
	person = Person{}
	results := []struct {
		Rs []neoReadStruct
	}{}
	query := &neoism.CypherQuery{
		Statement: `
                        MATCH (p:Person{uuid:{uuid}})<-[:HAS_MEMBER]-(m:Membership)
                        OPTIONAL MATCH (m)-[:HAS_ORGANISATION]->(o:Organisation)
                        OPTIONAL MATCH (m)-[rr:HAS_ROLE]->(r:Role)
                        WITH
                                { id:p.uuid, types:labels(p), prefLabel:p.prefLabel, labels:p.labels} as p,
                                { id:o.uuid, types:labels(o), prefLabel:o.prefLabel, labels:o.labels} as o,
                                { id:m.uuid, types:labels(m), prefLabel:m.prefLabel, title:m.title, changeEvents:[{startedAt:m.inceptionDate}, {endedAt:m.terminationDate}] } as m,
                                { id:r.uuid, types:labels(r), prefLabel:r.prefLabel, changeEvents:[{startedAt:m.inceptionDate}, {endedAt:m.terminationDate}] } as r
                        WITH p, m, o, collect(r) as r
                        WITH p, collect({m:m, o:o, r:r}) as m
                        RETURN collect ({p:p, m:m}) as rs
                `,
		Parameters: neoism.Props{"uuid": uuid},
		Result:     &results,
	}
	err = pcw.db.Cypher(query)
	if err != nil {
		log.Errorf("Error looking up uuid %s with query %s from neoism: %+v\n", uuid, query.Statement, err)
		return Person{}, false, fmt.Errorf("Error accessing datastore for uuid: %s", uuid)
	}
	log.Debugf("CypherResult ReadPeople for uuid: %s was: %+v", uuid, results[0].Rs[0])
	if (len(results)) == 0 {
		return Person{}, false, nil
	} else if len(results) != 1 && len(results[0].Rs) != 1 {
		errMsg := fmt.Sprintf("Multiple people found with the same uuid:%s !", uuid)
		log.Error(errMsg)
		return Person{}, true, errors.New(errMsg)
	}
	person = neoReadStructToPerson(results[0].Rs[0])
	log.Debugf("Returning %v", person)
	return person, true, nil
}

func neoReadStructToPerson(neo neoReadStruct) Person {
	public := Person{}
	public.Thing = &Thing{}
	public.ID = idURL(neo.P.ID)
	public.APIURL = apiURL(neo.P.ID, neo.P.Types)
	public.Types = typeURIs(neo.P.Types)
	public.PrefLabel = neo.P.PrefLabel
	if len(neo.P.Labels) > 0 {
		public.Labels = &neo.P.Labels
	}
	public.Memberships = make([]Membership, len(neo.M))
	for mIdx, neoMem := range neo.M {
		membership := Membership{}
		membership.Title = neoMem.M.PrefLabel
		membership.Organisation = Organisation{}
		membership.Organisation.Thing = &Thing{}
		membership.Organisation.ID = idURL(neoMem.O.ID)
		membership.Organisation.APIURL = apiURL(neoMem.O.ID, neoMem.O.Types)
		membership.Organisation.Types = typeURIs(neoMem.O.Types)
		membership.Organisation.PrefLabel = neoMem.O.PrefLabel
		if len(neoMem.O.Labels) > 0 {
			membership.Organisation.Labels = &neoMem.O.Labels
		}
		membership.ChangeEvents = changeEvent(neoMem.M.ChangeEvents)
		membership.Roles = make([]Role, len(neoMem.R))
		for rIdx, neoRole := range neoMem.R {
			role := Role{}
			role.Thing = &Thing{}
			role.ID = idURL(neoRole.ID)
			role.APIURL = apiURL(neoRole.ID, neoRole.Types)
			role.PrefLabel = neoRole.PrefLabel
			membership.ChangeEvents = changeEvent(neoRole.ChangeEvents)
			membership.Roles[rIdx] = role
		}
		public.Memberships[mIdx] = membership
	}
	log.Debugf("neoReadStructToPerson neo: %+v result: %+v", neo, public)
	return public
}

func changeEvent(neoChgEvts []neoChangeEvent) *[]ChangeEvent {
	if len(neoChgEvts) == 0 {
		return nil
	}
	var results []ChangeEvent
	for _, neoChgEvt := range neoChgEvts {
		if neoChgEvt.StartedAt != "" {
			results = append(results, ChangeEvent{StartedAt: neoChgEvt.StartedAt})
		}
		if neoChgEvt.EndedAt != "" {
			results = append(results, ChangeEvent{EndedAt: neoChgEvt.EndedAt})
		}
	}
	log.Debugf("changeEvent converted: %+v result:%+v", neoChgEvts, results)
	return &results
}

func apiURL(id string, types []string) string {
	base := "http://api.ft.com/"
	for _, t := range types {
		switch t {
		case "Person":
			return base + "people/" + id
		case "Organisation", "Company", "PublicCompany", "PrivateCompany":
			return base + "organisations/" + id
		}
	}
	return base + "things/" + id
}

func idURL(neoID string) string {
	return "http://api.ft.com/things/" + neoID
}

func typeURIs(neoTypes []string) []string {
	var results []string
	base := "http://www.ft.com/ontology/"
	for _, t := range neoTypes {
		switch t {
		case "Person":
			results = append(results, base+"person/Person")
		case "Organisation":
			results = append(results, base+"organisation/"+t)
		case "Company", "PublicCompany", "PrivateCompany":
			results = append(results, base+"company/"+t)
		}
	}
	log.Debugf("Converted types: %v to %v", neoTypes, results)
	return results
}
