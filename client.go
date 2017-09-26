// Client library for auth.bytemark.co.uk -  Thin wrapper around net/http that
// supports creating a session and reading it back
//
// Lots TODO.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// Client represents a client communicating with auth3
type Client struct {
	sessionEndpoint *url.URL
	HTTP            http.Client
}

// Credentials provides a number of credentials used to authenticate a session.
// We expect factor => credential, i.e:
// {"password" => "foo", "yubikey" => "cccbar"}
// FIXME: this is a bit of a lie as you also pass the username here
type Credentials map[string]string

// Error wraps an underlying error with a higher level message
type Error struct {
	Message string
	Err     error
}

// Error returns an error message with high and low level components
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

// SessionData collects the data held in a session
type SessionData struct {
	Token    string // not actually in the session, but communicate it here
	Username string
	Factors  []string

	// The groups this user is a member of
	GroupMemberships []string `json:"group_memberships"`
}

// New returns a newly initialised client.
func New(endpoint string) (*Client, error) {
	// ensure we end up with a string like "https://example.com/session"
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("Endpoint scheme must be http or https, got: %s", parsed.Scheme)
	}

	if len(parsed.Path) > 0 {
		if parsed.Path[len(parsed.Path):] != "/" {
			parsed.Path = parsed.Path + "/"
		}
	} else {
		parsed.Path = "/"
	}
	parsed.Path = parsed.Path + "session"

	return &Client{sessionEndpoint: parsed}, nil
}

// ReadSession takes a session token and returns the session data for that
// token, or nil is the session is invalid.
func (c *Client) ReadSession(ctx context.Context, token string) (*SessionData, error) {
	req, reqErr := http.NewRequest("GET", c.sessionURL(token), nil)
	if reqErr != nil {
		return nil, &Error{"Request couldn't be created", reqErr}
	}

	req.Header.Add("Accept", "application/json")
	body, bodyErr := c.doRequest(ctx, req)
	if bodyErr != nil {
		return nil, &Error{"Request failed", bodyErr}
	}
	if len(body) == 0 {
		return nil, &Error{"Empty body returned reading session", nil}
	}

	out := &SessionData{Token: token} // not included in session data
	jsonErr := json.Unmarshal(body, out)
	if jsonErr != nil {
		return nil, jsonErr
	}

	return out, nil
}

// CreateSessionToken creates a session and returns the token.
func (c *Client) CreateSessionToken(ctx context.Context, credentials Credentials) (string, error) {
	data, marshalErr := json.Marshal(credentials)
	if marshalErr != nil {
		return "", marshalErr
	}

	req, reqErr := http.NewRequest("POST", c.sessionEndpoint.String(), bytes.NewBuffer(data))
	if reqErr != nil {
		return "", &Error{"Couldn't create request", reqErr}
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "text/plain")

	body, err := c.doRequest(ctx, req)
	if err != nil || len(body) == 0 {
		return "", &Error{"Couldn't create session", err}
	}

	// FIXME: auth should really put the token in an Authorization: header.
	// TODO:  It could then return the session data in the response body.
	return string(body), nil
}

// CreateSession creates a session, returning the session data rather than
// just the token.
func (c *Client) CreateSession(ctx context.Context, credentials Credentials) (*SessionData, error) {

	token, createErr := c.CreateSessionToken(ctx, credentials)
	if createErr != nil {
		return nil, createErr
	}

	sessionData, getErr := c.ReadSession(ctx, token)
	if getErr != nil {
		return nil, getErr
	}

	return sessionData, nil
}

// CreateImpersonatedSessionToken uses an existing session token to create
// an impersonated session and returns its token.
func (c *Client) CreateImpersonatedSessionToken(ctx context.Context, token, username string) (string, error) {
	credentials := Credentials{
		"username": username,
	}
	data, marshalErr := json.Marshal(credentials)
	if marshalErr != nil {
		return "", marshalErr
	}

	req, reqErr := http.NewRequest("POST", c.sessionURL(token), bytes.NewBuffer(data))
	if reqErr != nil {
		return "", &Error{"Couldn't create request", reqErr}
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "text/plain")

	body, err := c.doRequest(ctx, req)
	if err != nil || len(body) == 0 {
		return "", &Error{"Couldn't create session", err}
	}

	return string(body), nil

}

// CreateImpersonatedSession uses an existing session token to create an
// impersonated session, returning the session data rather than just the token.
func (c *Client) CreateImpersonatedSession(ctx context.Context, token, user string) (*SessionData, error) {

	token, createErr := c.CreateImpersonatedSessionToken(ctx, token, user)
	if createErr != nil {
		return nil, createErr
	}

	sessionData, getErr := c.ReadSession(ctx, token)
	if getErr != nil {
		return nil, getErr
	}

	return sessionData, nil
}

// TODO: func (c *Client) CreateUser() {}
// TODO: func (c *Client) ReadUser() {}
// TODO: func (c *Client) IsUsernameAvailable(username string) {}
// TODO: func (c *Client) ResetUserPassword() {}

func errOrCtxErr(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}
	return err
}

func (c *Client) doRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	rsp, rspErr := c.HTTP.Do(req.WithContext(ctx))
	if rspErr != nil {
		return nil, errOrCtxErr(ctx, rspErr)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, errOrCtxErr(ctx, err)
	}

	if rsp.StatusCode < 200 || rsp.StatusCode > 299 {
		if len(body) == 0 {
			return nil, errors.New(rsp.Status)
		}
		return nil, errors.New(string(body))
	}

	return body, nil
}

func (c *Client) sessionURL(token string) string {
	x := *c.sessionEndpoint // shallow copy. Don't touch UserInfo
	x.Path = x.Path + "/" + token
	return x.String()
}
