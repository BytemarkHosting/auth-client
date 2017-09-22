// Simple auth client executable
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"gitlab.bytemark.co.uk/auth/client"
)

var (
	endpoint = flag.String("endpoint", "https://auth.bytemark.co.uk", "URL for an auth server")
	mode     = flag.String("mode", "", "What to do. Options: ReadSession, CreateSession, CreateImpersonated Session")
	token    = flag.String("token", "", "Token to use. Needed for ReadSession and CreateImpersonatedSession")
	username = flag.String("username", "", "Username. Needed for CreateSession and CreateImpersonatedSession")
	password = flag.String("password", "", "Password. Only needed for CreateSession")
	yubikey  = flag.String("yubikey", "", "Yubikey OTP. Only needed for CreateSession")
)

func main() {
	flag.Parse()
	auth, err := client.New(*endpoint)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx := context.TODO()

	switch *mode {
	case "ReadSession":
		session, err := auth.ReadSession(ctx, *token)
		fmt.Printf("Session: %+v, err: %+v\n", session, err)
	case "CreateSession":
		creds := client.Credentials{
			"username": *username,
			"password": *password,
		}
		if *yubikey != "" {
			creds["yubikey"] = *yubikey
		}
		session, err := auth.CreateSession(ctx, creds)
		fmt.Printf("Session: %+v, err: %+v\n", session, err)
	case "CreateImpersonatedSession":
		session, err := auth.CreateImpersonatedSession(ctx, *token, *username)
		fmt.Printf("Session: %+v, err: %+v\n", session, err)
	default:
		fmt.Printf("Unrecognised mode: %s\n", *mode)
		os.Exit(1)
	}
}
