package gitkit

import (
	"net/http"
)

type Credential struct {
	Username      string
	Password      string
	Authorization string
}

func getCredential(req *http.Request) Credential {
	cred := Credential{}

	user, pass, _ := req.BasicAuth()

	auth := req.Header.Get("Authorization")

	cred.Username = user
	cred.Password = pass
	cred.Authorization = auth

	return cred
}
