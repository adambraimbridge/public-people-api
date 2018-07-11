package people

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Financial-Times/go-logger"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/jarcoal/httpmock.v1"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	server    *httptest.Server
	personURL string
	isFound   bool
)

type HandlerTestSuite struct {
	suite.Suite
	mockDriver *MockDriver
	router     *mux.Router
	handler    *Handler
}

func (suite *HandlerTestSuite) SetupTest() {
	logger.InitDefaultLogger("handler-test")
	suite.router = mux.NewRouter()
	suite.mockDriver = &MockDriver{}
	suite.handler = NewHandler(0, "http://localhost")
	suite.handler.RegisterHandlers(suite.router)
}

func (suite *HandlerTestSuite) TestGetPeople_Success() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	uuid := "70f4732b-7f7d-30a1-9c29-0cceec23760e"
	fakeResponse, _ := ioutil.ReadFile("./fixtures/Public-Concepts-5c21e52f-48f9-3a77-ad36-318163f284be.json")

	url := "http://localhost/__public-concepts-api/concepts/" + uuid
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, string(fakeResponse)))

	person := Person{
		Thing: Thing{
			ID:        "http://api.ft.com/things/70f4732b-7f7d-30a1-9c29-0cceec23760e",
			APIURL:    "http://api.ft.com/people/70f4732b-7f7d-30a1-9c29-0cceec23760e",
			PrefLabel: "Someone",
		},
	}

	req := newRequest("GET", "/people/"+uuid, "")
	rec := httptest.NewRecorder()
	suite.router.ServeHTTP(rec, req)

	retPerson := Person{}
	json.NewDecoder(rec.Result().Body).Decode(&retPerson)
	suite.Equal(http.StatusOK, rec.Result().StatusCode)
	suite.Equal(person, retPerson)
}

func (suite *HandlerTestSuite) TestGetPeople_NotFound() {
	uuid := "70f4732b-7f7d-30a1-9c29-0cceec23760e"

	suite.mockDriver.On("Read", uuid, mock.Anything).Return(Person{}, false, nil)

	req := newRequest("GET", "/people/"+uuid, "")
	rec := httptest.NewRecorder()
	suite.router.ServeHTTP(rec, req)

	msg := &errMsg{
		Message: personNotFoundMsg,
	}
	returnMsg := &errMsg{}
	json.NewDecoder(rec.Result().Body).Decode(returnMsg)
	suite.Equal(msg, returnMsg)
	suite.Equal(http.StatusNotFound, rec.Result().StatusCode)
	suite.mockDriver.AssertExpectations(suite.T())

}

func (suite *HandlerTestSuite) TestGetPeople_BadRequest() {
	req := newRequest("GET", "/people/BOO", "")
	rec := httptest.NewRecorder()
	suite.router.ServeHTTP(rec, req)

	msg := &errMsg{
		Message: badRequestMsg,
	}

	returnMsg := &errMsg{}
	json.NewDecoder(rec.Result().Body).Decode(returnMsg)
	suite.Equal(msg, returnMsg)
	suite.Equal(http.StatusBadRequest, rec.Result().StatusCode)
	suite.mockDriver.AssertExpectations(suite.T())

}

func (suite *HandlerTestSuite) TestGetPeople_Redirect() {
	uuid := "70f4732b-7f7d-30a1-9c29-0cceec23760e"
	canonicalUUID := "dcd90ae4-52c2-4851-b5af-5c3d6ef527b6"

	person := Person{
		Thing: Thing{
			ID:        "http://api.ft.com/things/dcd90ae4-52c2-4851-b5af-5c3d6ef527b6",
			APIURL:    "http://api.ft.com/people/dcd90ae4-52c2-4851-b5af-5c3d6ef527b6",
			PrefLabel: "Someone",
		},
	}

	suite.mockDriver.On("Read", uuid, mock.Anything).Return(person, true, nil)

	req := newRequest("GET", "/people/"+uuid, "")
	rec := httptest.NewRecorder()
	suite.router.ServeHTTP(rec, req)

	msg := &errMsg{
		Message: fmt.Sprintf(redirectedPerson, uuid, canonicalUUID),
	}

	returnMsg := &errMsg{}
	json.NewDecoder(rec.Result().Body).Decode(returnMsg)
	suite.Equal(msg, returnMsg)

	suite.Equal(http.StatusMovedPermanently, rec.Result().StatusCode)
	suite.mockDriver.AssertExpectations(suite.T())
}

func (suite *HandlerTestSuite) TestGetPeople_InternalError() {
	uuid := "70f4732b-7f7d-30a1-9c29-0cceec23760e"

	suite.mockDriver.On("Read", uuid, mock.Anything).Return(Person{}, false, errors.New("Some error"))

	req := newRequest("GET", "/people/"+uuid, "")
	rec := httptest.NewRecorder()
	suite.router.ServeHTTP(rec, req)

	msg := &errMsg{
		Message: personUnableToBeRetrieved,
	}

	returnMsg := &errMsg{}
	json.NewDecoder(rec.Result().Body).Decode(returnMsg)
	suite.Equal(msg, returnMsg)

	suite.Equal(http.StatusInternalServerError, rec.Result().StatusCode)
	suite.mockDriver.AssertExpectations(suite.T())

}

func (suite *HandlerTestSuite) TestGetPeople_MethodNotAllowedOnPost() {
	uuid := "70f4732b-7f7d-30a1-9c29-0cceec23760e"
	req := newRequest("POST", "/people/"+uuid, "")
	rec := httptest.NewRecorder()
	suite.router.ServeHTTP(rec, req)
	suite.Equal(http.StatusMethodNotAllowed, rec.Result().StatusCode)
	suite.mockDriver.AssertExpectations(suite.T())

}

func TestHandlersTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func newRequest(method, url string, body string) *http.Request {
	var payload io.Reader
	if body != "" {
		payload = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		panic(err)
	}
	return req
}

type errMsg struct {
	Message string `json:"message"`
}
