package people

import (
	"fmt"
	"time"

	"github.com/Financial-Times/neo-model-utils-go/mapper"
	"github.com/Financial-Times/neo-utils-go/neoutils"
	"github.com/jmcvetta/neoism"
	log "github.com/sirupsen/logrus"
)

// Driver interface
type Driver interface {
	Read(id string, transactionID string) (person Person, found bool, err error)
	CheckConnectivity() error
}

// CypherDriver struct
type CypherDriver struct {
	conn neoutils.NeoConnection
	env  string
}

//NewCypherDriver instantiate driver
func NewCypherDriver(conn neoutils.NeoConnection, env string) CypherDriver {
	return CypherDriver{conn, env}
}

// CheckConnectivity tests neo4j by running a simple cypher query
func (pcw CypherDriver) CheckConnectivity() error {
	return neoutils.Check(pcw.conn)
}

type neoChangeEvent struct {
	StartedAt string
	EndedAt   string
}

type neoReadStruct struct {
	P struct {
		ID              string
		Types           []string
		DirectType      string
		PrefLabel       string
		Labels          []string
		Salutation      string
		BirthYear       int
		EmailAddress    string
		TwitterHandle   string
		FacebookProfile string
		LinkedinProfile string
		Description     string
		DescriptionXML  string
		ImageURL        string
	}
	M []struct {
		M struct {
			ID           string
			Types        []string
			DirectType   string
			PrefLabel    string
			Title        string
			ChangeEvents []neoChangeEvent
		}
		O struct {
			ID         string
			Types      []string
			DirectType string
			PrefLabel  string
			Labels     []string
		}
		R []struct {
			ID           string
			Types        []string
			DirectType   string
			PrefLabel    string
			ChangeEvents []neoChangeEvent
		}
	}
}

func (pcw CypherDriver) Read(uuid string, transactionID string) (Person, bool, error) {
	person := Person{}
	results := []struct {
		Rs []neoReadStruct
	}{}

	query := &neoism.CypherQuery{
		Statement: `
                        MATCH (identifier:UPPIdentifier{value:{uuid}})
                        MATCH (identifier)-[:IDENTIFIES]->(pp:Person)-[:EQUIVALENT_TO]->(canonical:Person)
						MATCH (canonical)<-[:EQUIVALENT_TO]-(p:Person)
                        OPTIONAL MATCH (p)<-[:HAS_MEMBER]-(m:Membership)
                        OPTIONAL MATCH (m)-[:HAS_ORGANISATION]->(o:Organisation)
                        OPTIONAL MATCH (m)-[rr:HAS_ROLE]->(r:MembershipRole)
                        WITH    canonical,
                                { id:o.uuid, types:labels(o), prefLabel:o.prefLabel} as o,
                                { id:m.uuid, types:labels(m), prefLabel:m.prefLabel, title:m.title, changeEvents:[{startedAt:m.inceptionDate}, {endedAt:m.terminationDate}] } as m,
                                { id:r.uuid, types:labels(r), prefLabel:r.prefLabel, changeEvents:[{startedAt:rr.inceptionDate}, {endedAt:rr.terminationDate}] } as r
                        WITH canonical, m, o, collect(r) as r ORDER BY o.uuid DESC
                        WITH canonical, collect({m:m, o:o, r:r}) as m
                        WITH m, { ID:canonical.prefUUID, types:labels(canonical), prefLabel:canonical.prefLabel, labels:canonical.aliases,
								birthYear:canonical.birthYear, salutation:canonical.salutation, emailAddress:canonical.emailAddress,
								twitterHandle:canonical.twitterHandle, facebookProfile:canonical.facebookProfile, linkedinProfile:canonical.linkedinProfile,
								imageUrl:canonical.imageUrl, Description:canonical.description, descriptionXML:canonical.descriptionXML} as p
                        RETURN collect ({p:p, m:m}) as rs
                        `,
		Parameters: neoism.Props{"uuid": uuid},
		Result:     &results,
	}

	err := pcw.conn.CypherBatch([]*neoism.CypherQuery{query})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"UUID": uuid, "transaction_id": transactionID}).Error("Error Querying Neo4J for a Person")
		return Person{}, true, err
	}

	if len(results) == 0 || (len(results[0].Rs) == 0 || results[0].Rs[0].P.ID == "") {
		p, f, e := pcw.ReadOldConcordanceModel(uuid, transactionID)
		return p, f, e
	} else if len(results) != 1 {
		err := fmt.Errorf("Multiple people found with the same uuid: %s", uuid)
		log.WithFields(log.Fields{"UUID": uuid, "transaction_id": transactionID}).Error(err.Error())
		return Person{}, true, err
	}

	person = neoReadStructToPerson(results[0].Rs[0], pcw.env)
	return person, true, nil
}

func (pcw CypherDriver) ReadOldConcordanceModel(uuid string, transactionID string) (person Person, found bool, err error) {
	person = Person{}
	results := []struct {
		Rs []neoReadStruct
	}{}

	query := &neoism.CypherQuery{
		Statement: `
                        MATCH (identifier:UPPIdentifier{value:{uuid}})
                        MATCH (identifier)-[:IDENTIFIES]->(p:Person)
                        OPTIONAL MATCH (p)<-[:HAS_MEMBER]-(m:Membership)
                        OPTIONAL MATCH (m)-[:HAS_ORGANISATION]->(o:Organisation)
                        OPTIONAL MATCH (m)-[rr:HAS_ROLE]->(r:MembershipRole)
                        WITH    p,
                                { id:o.uuid, types:labels(o), prefLabel:o.prefLabel} as o,
                                { id:m.uuid, types:labels(m), prefLabel:m.prefLabel, title:m.title, changeEvents:[{startedAt:m.inceptionDate}, {endedAt:m.terminationDate}] } as m,
                                { id:r.uuid, types:labels(r), prefLabel:r.prefLabel, changeEvents:[{startedAt:rr.inceptionDate}, {endedAt:rr.terminationDate}] } as r
                        WITH p, m, o, collect(r) as r ORDER BY o.uuid DESC
                        WITH p, collect({m:m, o:o, r:r}) as m
                        WITH m, { id:p.uuid, types:labels(p), prefLabel:p.prefLabel, labels:p.aliases,
												     birthYear:p.birthYear, salutation:p.salutation, emailAddress:p.emailAddress,
														 twitterHandle:p.twitterHandle, facebookProfile:p.facebookProfile, linkedinProfile:p.linkedinProfile,
														 imageUrl:p.imageUrl, Description:p.description, descriptionXML:p.descriptionXML} as p
                        RETURN collect ({p:p, m:m}) as rs
                        `,
		Parameters: neoism.Props{"uuid": uuid},
		Result:     &results,
	}

	err = pcw.conn.CypherBatch([]*neoism.CypherQuery{query})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"UUID": uuid, "transaction_id": transactionID}).Error("Query execution failed")
		return Person{}, false, err
	} else if len(results) == 0 || len(results[0].Rs) == 0 {
		log.WithFields(log.Fields{"UUID": uuid, "transaction_id": transactionID}).Info("Person not found")
		return Person{}, false, nil
	} else if len(results) != 1 && len(results[0].Rs) != 1 {
		err = fmt.Errorf("Multiple people found with the same uuid:%s !", uuid)
		log.WithFields(log.Fields{"UUID": uuid, "transaction_id": transactionID}).Error(err.Error())
		return Person{}, true, err
	}

	person = neoReadStructToPerson(results[0].Rs[0], pcw.env)
	return person, true, nil
}

func neoReadStructToPerson(neo neoReadStruct, env string) Person {
	public := Person{}
	public.Thing = Thing{}
	public.ID = mapper.IDURL(neo.P.ID)
	public.APIURL = mapper.APIURL(neo.P.ID, neo.P.Types, env)
	public.Types = mapper.TypeURIs(neo.P.Types)
	public.DirectType = filterToMostSpecificType(neo.P.Types)
	public.PrefLabel = neo.P.PrefLabel
	if len(neo.P.Labels) > 0 {
		public.Labels = neo.P.Labels
	}
	public.BirthYear = neo.P.BirthYear
	public.Salutation = neo.P.Salutation
	public.Description = neo.P.Description
	public.DescriptionXML = neo.P.DescriptionXML
	public.EmailAddress = neo.P.EmailAddress
	public.TwitterHandle = neo.P.TwitterHandle
	public.FacebookProfile = neo.P.FacebookProfile
	public.ImageURL = neo.P.ImageURL

	if len(neo.M) > 0 {
		memberships := []Membership{}
		for _, neoMem := range neo.M {
			if neoMem.M.ID != "" && neoMem.O.ID != "" && len(neoMem.R) > 0 {
				membership := Membership{}
				membership.Title = neoMem.M.PrefLabel
				membership.Types = mapper.TypeURIs(neoMem.M.Types)
				membership.DirectType = filterToMostSpecificType(neoMem.M.Types)
				membership.Organisation = Organisation{}
				membership.Organisation.Thing = Thing{}
				membership.Organisation.ID = mapper.IDURL(neoMem.O.ID)
				membership.Organisation.APIURL = mapper.APIURL(neoMem.O.ID, neoMem.O.Types, env)
				membership.Organisation.Types = mapper.TypeURIs(neoMem.O.Types)
				membership.Organisation.DirectType = filterToMostSpecificType(neoMem.O.Types)
				membership.Organisation.PrefLabel = neoMem.O.PrefLabel
				if len(neoMem.O.Labels) > 0 {
					membership.Organisation.Labels = neoMem.O.Labels
				}
				if a, b := changeEvent(neoMem.M.ChangeEvents); a == true {
					membership.ChangeEvents = b
				}

				roles := []Role{}
				for _, neoRole := range neoMem.R {
					if neoRole.ID != "" {
						role := Role{}
						role.Thing = Thing{}
						role.ID = mapper.IDURL(neoRole.ID)
						role.APIURL = mapper.APIURL(neoRole.ID, neoRole.Types, env)
						role.Types = mapper.TypeURIs(neoRole.Types)
						role.DirectType = filterToMostSpecificType(neoRole.Types)
						role.PrefLabel = neoRole.PrefLabel
						if a, b := changeEvent(neoRole.ChangeEvents); a == true {
							role.ChangeEvents = b
						}
						roles = append(roles, role)
					}
				}
				if len(roles) > 0 {
					membership.Roles = roles
					memberships = append(memberships, membership)
				}
			}
			public.Memberships = memberships
		}
	}

	return public
}

func changeEvent(neoChgEvts []neoChangeEvent) (bool, []ChangeEvent) {
	var results []ChangeEvent
	currentLayout := "2006-01-02T15:04:05.999Z"
	layout := "2006-01-02T15:04:05Z"

	if neoChgEvts[0].StartedAt == "" && neoChgEvts[1].EndedAt == "" {
		results = make([]ChangeEvent, 0, 0)
		return false, results
	}
	for _, neoChgEvt := range neoChgEvts {
		if neoChgEvt.StartedAt != "" {
			t, _ := time.Parse(currentLayout, neoChgEvt.StartedAt)
			results = append(results, ChangeEvent{StartedAt: t.Format(layout)})
		}
		if neoChgEvt.EndedAt != "" {
			t, _ := time.Parse(layout, neoChgEvt.EndedAt)
			results = append(results, ChangeEvent{EndedAt: t.Format(layout)})
		}
	}
	return true, results
}

func filterToMostSpecificType(unfilteredTypes []string) string {
	mostSpecificType, err := mapper.MostSpecificType(unfilteredTypes)
	if err != nil {
		return ""
	}
	fullURI := mapper.TypeURIs([]string{mostSpecificType})
	return fullURI[0]
}
