package client_test

import (
	. "."
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"testing"
)

type TestSuite struct {
	ts     *httptest.Server
	client *Client
}

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

// This handler just blocks for a second
func SlowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(1 * time.Second)
}

// Uses the above two vars to answer auth questions like a real auth server.
func FixturesHandler(w http.ResponseWriter, r *http.Request) {
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

func withHandledClient(t *testing.T, h func(w http.ResponseWriter, r *http.Request), f func(client *Client)) {
	ts := httptest.NewServer(http.HandlerFunc(h))
	defer ts.Close()
	client, err := New(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	f(client)
}

func withTestClient(t *testing.T, f func(client *Client)) {
	withHandledClient(t, FixturesHandler, f)
}

func withSlowTestClient(t *testing.T, f func(client *Client)) {
	withHandledClient(t, SlowHandler, f)
}

func TestNewRejectsNonHTTPchemes(t *testing.T) {
	x, err := New("ftp://example.com")
	if x != nil {
		t.Error("did not expect a client to be returned")
	}
	if err == nil {
		t.Fatal("expected an error to be returned")
	}
	if !strings.Contains(err.Error(), "scheme") {
		t.Error("unexpected error: %s", err.Error())
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

func cmpSession(t *testing.T, a, b *SessionData) {
	if a.Token != b.Token {
		t.Error("unexpected Token %s", a.Token)
	}
	if a.Username != b.Username {
		t.Error("unexpected Username %s", a.Token)
	}
	if !cmpStringArrays(a.Factors, b.Factors) {
		t.Error("unexpected Factors %v", a.Factors)
	}
	if !cmpStringArrays(a.GroupMemberships, b.GroupMemberships) {
		t.Error("unexpected GroupMemberships %v", a.Factors)
	}
}

func TestReadSession(t *testing.T) {
	withTestClient(t, func(client *Client) {
		session, err := client.ReadSession(context.Background(), "good-session")
		if err != nil {
			t.Fatal(err)
		}
		cmpSession(t, session, fSessions["good-session"])
	})
}

func TestReadSessionCancellation(t *testing.T) {
	withSlowTestClient(t, func(client *Client) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		session, err := client.ReadSession(ctx, "good-session")
		if session != nil {
			t.Error("no session should be returned")
		}
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Error("unexpected error: %v", err)
		}
	})
}

func (s *TestSuite) TestCreateSession(t *testing.T) {
	withTestClient(t, func(client *Client) {
		session, err := client.CreateSession(context.Background(), fCreds["good-user"])
		if err != nil {
			t.Fatal(err)
		}
		cmpSession(t, session, fSessions["good-session"])
	})
}

func TestCreateSessionCancellation(t *testing.T) {
	withSlowTestClient(t, func(client *Client) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		session, err := client.CreateSession(ctx, fCreds["good-user"])
		if session != nil {
			t.Error("no session should be returned")
		}
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Error("unexpected error: %v", err)
		}
	})
}

func TestCreateSessionWithBadCredentials(t *testing.T) {
	withTestClient(t, func(client *Client) {
		session, err := client.CreateSession(context.Background(), Credentials{"username": "bad-user", "password": "foo"})
		if err == nil {
			t.Error("expected an error")
		}
		if session != nil {
			t.Error("no session should be returned")
		}
	})
}

func TestCreateSessionToken(t *testing.T) {
	withTestClient(t, func(client *Client) {
		token, err := client.CreateSessionToken(context.Background(), fCreds["good-user"])
		if err != nil {
			t.Fatal(err)
		}
		if token != fSessions["good-session"].Token {
			t.Error("unexpected token %s", token)
		}
	})
}

func TestCreateSessionTokenCancellation(t *testing.T) {
	withSlowTestClient(t, func(client *Client) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		token, err := client.CreateSessionToken(ctx, fCreds["good-user"])
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
	withTestClient(t, func(client *Client) {
		session, err := client.CreateSession(context.Background(), Credentials{"username": "bad-user", "password": "foo"})
		if err == nil {
			t.Error("expected an error")
		}
		if session != nil {
			t.Error("no session should be returned")
		}
	})
}
