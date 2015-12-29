package main

import (
	"log"

	"github.com/Financial-Times/neoism"
)

// PeopleDriver interface
type PeopleDriver interface {
	Read(id string) map[string]interface{}
}

// PeopleCypherDriver struct
type PeopleCypherDriver struct {
	db *neoism.Database
}

//NewPeopleCypherDriver instanciate driver
func NewPeopleCypherDriver(db *neoism.Database) PeopleCypherDriver {
	return PeopleCypherDriver{db}
}

func (pcw PeopleCypherDriver) Read(uuid string) map[string]interface{} {

	results := []struct {
		P *neoism.Node
	}{}

	query := &neoism.CypherQuery{
		Statement: `
                        MATCH (p:Person {uuid: {uuid}})
                        RETURN p
                        `,
		Parameters:   neoism.Props{"uuid": uuid},
		Result:       &results,
		IncludeStats: true,
	}

	err := pcw.db.Cypher(query)
	if err != nil {
		panic(err)
	}

	// log.Println(query.Statement)
	// log.Printf("Returned structure %+v\n", results[0])
	// log.Printf("Labels %+v Data %+v Relationships %+v \n", result[0].N.Labels(), result[0].Data, result[0].HrefAllTypedRels)

	result := make(map[string]interface{})
	results[0].P.Db = pcw.db
	Thing(results[0].P, &result)
	log.Printf("Returning %+v\n", result)
	return result
}
