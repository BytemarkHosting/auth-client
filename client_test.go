package client_test

import (
	. "."
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "gopkg.in/check.v1"
	"testing"
)

type TestSuite struct {
	ts     *httptest.Server
	client *Client
}

var _ = Suite(&TestSuite{})

func Test(t *testing.T) { TestingT(t) }

// FIXME: test concurrency=1 as a result of using globals here
var fCreds = map[string]Credentials{
	"good-user": Credentials{"username": "good-user", "password": "foo"},
}

var fSessions = map[string]*SessionData{
	"good-session": &SessionData{
		Token:            "good-session",
		Username:         "foo",
		Factors:          []string{"password", "google-auth"},
		GroupMemberships: []string{"staff"},
	},
}

// Uses the above two vars to answer auth questions like a real auth server.
func FixturesHandler(s *TestSuite, c *C, w http.ResponseWriter, r *http.Request) {
	pathBits := strings.Split(r.URL.Path, "/")
	switch r.Method {
	case "POST":
		switch r.URL.Path {
		case "/session":
			if r.Header.Get("Content-Type") == "application/json" {
				bodyCreds := make(Credentials)
				data := make([]byte, 4096)
				r, err := r.Body.Read(data)
				if r == 0 || (err != nil && err != io.EOF) {
					w.WriteHeader(400)
					w.Write([]byte("Error reading body: " + err.Error()))
					return
				}
				jErr := json.Unmarshal(data[0:r], &bodyCreds)
				if jErr != nil {
					w.WriteHeader(400)
					w.Write([]byte("Error parsing body to JSON: " + jErr.Error()))
					return
				}
				ourCreds := fCreds[bodyCreds["username"]]
				if ourCreds != nil {
					if ourCreds["password"] != bodyCreds["password"] {
						w.WriteHeader(403)
						return
					}
					w.Write([]byte("good-session"))
					return
				}
				w.WriteHeader(403)
				return
			} else {
				w.WriteHeader(400)
				w.Write([]byte(`Bad content-type`))
				return
			}
		default:
			w.WriteHeader(404)
			return
		}
	case "GET":
		switch pathBits[1] {
		case "session":
			d := fSessions[pathBits[2]]
			if d == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Add("Content-Type", "application/json")
			// We construct our own json here. The token is not included in the output.
			w.Write([]byte(
				`{"username":"` + d.Username +
					`","factors":["` + strings.Join(d.Factors, `","`) + `"],` +
					`"group_memberships":["` + strings.Join(d.GroupMemberships, `","`) +
					`"]}`,
			))
			return
		default:
			w.WriteHeader(404)
			return
		}
	default:
		w.WriteHeader(405)
		return
	}
}

// Invariant server, so start it once
func (s *TestSuite) SetUpSuite(c *C) {
	s.ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { FixturesHandler(s, c, w, r) }))

}

func (s *TestSuite) TearDownSuite(c *C) {
	if s.ts != nil {
		s.ts.Close()
	}
}

// New client per test though
func (s *TestSuite) SetUpTest(c *C) {
	client, err := New(s.ts.URL)
	c.Assert(err, IsNil)
	s.client = client
}

func (s *TestSuite) TestNewRejectsNonHTTPchemes(c *C) {
	x, err := New("ftp://example.com")
	c.Assert(x, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, ".*scheme.*")
}

func (s *TestSuite) TestHandlesTrickyEndpointURLs(c *C) {

}

func (s *TestSuite) TestReadSession(c *C) {
	session, err := s.client.ReadSession("good-session")
	c.Assert(err, IsNil)
	c.Assert(session, DeepEquals, fSessions["good-session"])
}

func (s *TestSuite) TestCreateSession(c *C) {
	session, err := s.client.CreateSession(fCreds["good-user"])
	c.Assert(err, IsNil)
	c.Assert(session, DeepEquals, fSessions["good-session"])
}

func (s *TestSuite) TestCreateSessionWithBadCredentials(c *C) {
	session, err := s.client.CreateSession(Credentials{"username": "bad-user", "password": "foo"})
	c.Assert(err, NotNil)
	c.Assert(session, IsNil)
}

func (s *TestSuite) TestCreateSessionToken(c *C) {
	token, err := s.client.CreateSessionToken(fCreds["good-user"])
	c.Assert(err, IsNil)
	c.Assert(token, Equals, "good-session")
}

func (s *TestSuite) TestCreateSessionTokenWithBadCredentials(c *C) {
	token, err := s.client.CreateSession(Credentials{"username": "bad-user", "password": "foo"})
	c.Assert(err, NotNil)
	c.Assert(token, IsNil)
}
