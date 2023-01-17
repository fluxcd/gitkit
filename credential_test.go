package gitkit

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getCredential(t *testing.T) {
	req, _ := http.NewRequest("get", "http://localhost", nil)
	cred := getCredential(req)
	assert.Equal(t, cred.Authorization, "")

	req, _ = http.NewRequest("get", "http://localhost", nil)
	req.SetBasicAuth("Alladin", "OpenSesame")
	cred = getCredential(req)

	assert.Equal(t, "Alladin", cred.Username)
	assert.Equal(t, "OpenSesame", cred.Password)
	assert.Contains(t, cred.Authorization, "Basic ")

	req, _ = http.NewRequest("get", "http://localhost", nil)
	req.Header.Add("Authorization", "Bearer VerySecretToken")
	cred = getCredential(req)

	assert.Equal(t, "Bearer VerySecretToken", cred.Authorization)
}
