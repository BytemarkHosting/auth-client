package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.bytemark.co.uk/auth/client"
)

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
			t.Errorf("unexpected error: %v", err)
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

func TestCreateImpersonatedSession(t *testing.T) {
	withTestClient(t, func(c *client.Client) {
		session, err := c.CreateImpersonatedSession(context.Background(), "good-session", "impersonated")
		if err != nil {
			t.Fatal(err)
		}
		cmpSession(t, session, fSessions["impersonated-session"])
	})
}
