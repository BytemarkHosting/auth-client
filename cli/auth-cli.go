// Simple auth client executable
package main

import (
	client ".."
	"flag"
	"fmt"
	"os"
)

var (
	endpoint = flag.String("endpoint", "https://auth.bytemark.co.uk", "URL for an auth server")
	mode     = flag.String("mode", "", "What to do. Options: ReadSession, CreateSession")
	token    = flag.String("token", "", "Token to use. Only needed for ReadSession")
	username = flag.String("username", "", "Username. Only needed for CreateSession")
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

	switch *mode {
	case "ReadSession":
		session, err := auth.ReadSession(*token)
		fmt.Printf("Session: %+v, err: %+v\n", session, err)
	case "CreateSession":
		creds := make(client.Credentials)
		creds["username"] = *username
		creds["password"] = *password
		if *yubikey != "" {
			creds["yubikey"] = *yubikey
		}
		session, err := auth.CreateSession(creds)
		fmt.Printf("Session: %+v, err: %+v\n", session, err)
	default:
		fmt.Printf("Unrecognised mode: %s\n", *mode)
		os.Exit(1)
	}
}
