package request

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginRequest(t *testing.T) {
	expectedMap := map[string]string{"email": "email", "password": "***"}
	expected, _ := json.Marshal(expectedMap)
	loginReq := LoginRequest{Email: "email", Password: "password"}

	actual, _ := json.Marshal(loginReq)

	assert.EqualValues(t, expected, actual)
	assert.EqualValues(t, "password", loginReq.Password)
}
