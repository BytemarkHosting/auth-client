package client_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.bytemark.co.uk/auth/client"
)

// FIXME: test concurrency=1 as a result of using globals here
var fCreds = map[string]client.Credentials{
	"good-user": client.Credentials{"username": "good-user", "password": "foo"},
}

var fSessions = map[string]*client.SessionData{
	"good-session": &client.SessionData{
		Token:            "good-session",
		Username:         "foo",
		Factors:          []string{"password", "google-auth"},
		GroupMemberships: []string{"staff"},
	},
	"impersonated-session": &client.SessionData{
		Token:            "impersonated-session",
		Username:         "bar",
		Factors:          []string{"impersonated"},
		GroupMemberships: []string{"wibble"},
	},
}

// This handler just blocks for a second
func SlowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(1 * time.Second)
}

func getCreds(w http.ResponseWriter, req *http.Request) (client.Credentials, bool) {
	bodyCreds := make(client.Credentials)
	data := make([]byte, 4096)
	r, err := req.Body.Read(data)
	if r == 0 || (err != nil && err != io.EOF) {
		http.Error(w, "Error reading body: "+err.Error(), 400)
		return bodyCreds, false
	}
	jErr := json.Unmarshal(data[0:r], &bodyCreds)
	if jErr != nil {
		http.Error(w, "Error parsing body to JSON: "+jErr.Error(), 400)
		return bodyCreds, false
	}
	return bodyCreds, true
}

func stringResponse(w http.ResponseWriter, resp string) {
	_, _ = w.Write([]byte(resp))
}

func fixturesPostHandler(w http.ResponseWriter, r *http.Request, pathBits []string) {

	switch pathBits[1] {
	case "session":
		if r.Header.Get("Content-Type") == "application/json" {
			bodyCreds, ok := getCreds(w, r)
			if !ok {
				return
			}
			if len(pathBits) == 2 {
				ourCreds := fCreds[bodyCreds["username"]]
				if ourCreds != nil {
					if ourCreds["password"] != bodyCreds["password"] {
						w.WriteHeader(403)
						return
					}
					stringResponse(w, "good-session")
					return
				}
			} else {
				d := fSessions[pathBits[2]]
				if d == nil {
					w.WriteHeader(403)
					return
				}
				stringResponse(w, "impersonated-session")
				return
			}
			w.WriteHeader(403)
			return
		} else {
			http.Error(w, `Bad content-type`, 400)
			return
		}
	default:
		w.WriteHeader(404)
		return
	}
}

func fixturesGetHandler(w http.ResponseWriter, r *http.Request, pathBits []string) {

	switch pathBits[1] {
	case "session":
		d := fSessions[pathBits[2]]
		if d == nil {
			w.WriteHeader(404)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		// We construct our own json here. The token is not included in the output.
		stringResponse(w, `{"username":"`+d.Username+`","factors":["`+strings.Join(d.Factors, `","`)+`"],`+`"group_memberships":["`+strings.Join(d.GroupMemberships, `","`)+`"]}`)
		return
	default:
		w.WriteHeader(404)
		return
	}
}

// FixturesHandler is a dummy service that responds like an auth server for
// the limited calls made in these tests.
func FixturesHandler(w http.ResponseWriter, r *http.Request) {
	pathBits := strings.Split(r.URL.Path, "/")
	switch r.Method {
	case "POST":
		fixturesPostHandler(w, r, pathBits)
	case "GET":
		fixturesGetHandler(w, r, pathBits)
	default:
		w.WriteHeader(405)
		return
	}
}
