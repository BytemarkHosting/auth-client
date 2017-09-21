package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.bytemark.co.uk/auth/client"
)

// FIXME: test concurrency=1 as a result of using globals here
var fCreds = map[string]client.Credentials{
	"good-user":    client.Credentials{"username": "good-user", "password": "foo"},
	"another-user": client.Credentials{"username": "another-user"},
}

var fSessions = map[string]*client.SessionData{
	"good-session": &client.SessionData{
		Token:            "good-session",
		Username:         "foo",
		Factors:          []string{"password", "google-auth"},
		GroupMemberships: []string{"staff"},
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

// Uses the above two vars to answer auth questions like a real auth server.
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

func withHandledClient(t *testing.T, h func(w http.ResponseWriter, r *http.Request), f func(client *client.Client)) {
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()
	client, err := client.New(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	f(client)
}

func withTestClient(t *testing.T, f func(client *client.Client)) {
	withHandledClient(t, FixturesHandler, f)
}

func withSlowTestClient(t *testing.T, f func(client *client.Client)) {
	withHandledClient(t, SlowHandler, f)
}

func TestNewRejectsNonHTTPchemes(t *testing.T) {
	x, err := client.New("ftp://example.com")
	if x != nil {
		t.Error("did not expect a client to be returned")
	}
	if err == nil {
		t.Fatal("expected an error to be returned")
	}
	if !strings.Contains(err.Error(), "scheme") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestHandlesTrickyEndpointURLs(t *testing.T) {
	t.Skip("TODO")
}

func cmpStringArrays(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cmpSession(t *testing.T, a, b *client.SessionData) {
	if a.Token != b.Token {
		t.Errorf("unexpected Token %s", a.Token)
	}
	if a.Username != b.Username {
		t.Errorf("unexpected Username %s", a.Token)
	}
	if !cmpStringArrays(a.Factors, b.Factors) {
		t.Errorf("unexpected Factors %v", a.Factors)
	}
	if !cmpStringArrays(a.GroupMemberships, b.GroupMemberships) {
		t.Errorf("unexpected GroupMemberships %v", a.Factors)
	}
}

func TestReadSession(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		session, err := c.ReadSession(context.Background(), "good-session")
		if err != nil {
			t.Fatal(err)
		}
		cmpSession(t, session, fSessions["good-session"])
	})
}

func TestReadSessionCancellation(t *testing.T) {
	withSlowTestClient(t, func(c *client.Client) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		session, err := c.ReadSession(ctx, "good-session")
		if session != nil {
			t.Error("no session should be returned")
		}
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCreateSession(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		session, err := c.CreateSession(context.Background(), fCreds["good-user"])
		if err != nil {
			t.Fatal(err)
		}
		cmpSession(t, session, fSessions["good-session"])
	})
}

func TestCreateSessionCancellation(t *testing.T) {
	withSlowTestClient(t, func(c *client.Client) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		session, err := c.CreateSession(ctx, fCreds["good-user"])
		if session != nil {
			t.Error("no session should be returned")
		}
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCreateSessionWithBadCredentials(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		session, err := c.CreateSession(context.Background(), client.Credentials{"username": "bad-user", "password": "foo"})
		if err == nil {
			t.Error("expected an error")
		}
		if session != nil {
			t.Error("no session should be returned")
		}
	})
}

func TestCreateSessionToken(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		token, err := c.CreateSessionToken(context.Background(), fCreds["good-user"])
		if err != nil {
			t.Fatal(err)
		}
		if token != fSessions["good-session"].Token {
			t.Errorf("unexpected token %s", token)
		}
	})
}

func TestCreateSessionTokenCancellation(t *testing.T) {
	withSlowTestClient(t, func(c *client.Client) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		token, err := c.CreateSessionToken(ctx, fCreds["good-user"])
		if token != "" {
			t.Error("no token should be returned")
		}
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Error("unexpected error: %v", err)
		}
	})
}

func TestCreateSessionTokenWithBadCredentials(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		session, err := c.CreateSession(context.Background(), client.Credentials{"username": "bad-user", "password": "foo"})
		if err == nil {
			t.Error("expected an error")
		}
		if session != nil {
			t.Error("no session should be returned")
		}
	})
}

func TestCreateImpersonatedSessionTokenWithGoodToken(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		token, err := c.CreateImpersonatedSessionToken(context.Background(), "good-session", "impersonated")
		if err != nil {
			t.Fatal(err)
		}
		if token != "impersonated-session" {
			t.Errorf("unexpected token %s", token)
		}
	})
}

func TestCreateImpersonatedSessionTokenWithBadToken(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		token, err := c.CreateImpersonatedSessionToken(context.Background(), "bad-session", "impersonated")
		if err == nil {
			t.Error("expected an error")
		}
		if token != "" {
			t.Error("no token should be returned")
		}
	})
}
