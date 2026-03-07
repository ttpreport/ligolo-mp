//go:build !windows

package connectproxy

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/go-ntlmssp"
)

type ntlmAuth struct {
	domain   string
	username string
	password string
}

// AuthNTLM returns a ProxyAuthorization that performs NTLM authentication
// using explicit credentials. Works on all platforms.
func AuthNTLM(domain, username, password string) ProxyAuthorization {
	return &ntlmAuth{domain: domain, username: username, password: password}
}

// AuthNegotiate on non-Windows falls back to NTLM with explicit credentials.
// SSO is not available without SSPI; returns nil if no credentials supplied.
func AuthNegotiate(domain, username, password string) ProxyAuthorization {
	if username == "" {
		return nil
	}
	return AuthNTLM(domain, username, password)
}

func (a *ntlmAuth) Type() string { return "NTLM" }

func (a *ntlmAuth) InitialResponse() string {
	neg, err := ntlmssp.NewNegotiateMessage(a.domain, "")
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(neg)
}

func (a *ntlmAuth) ChallengeResponse(challenge string) string {
	challengeBytes, err := base64.StdEncoding.DecodeString(challenge)
	if err != nil {
		return ""
	}
	user := a.username
	if a.domain != "" {
		user = fmt.Sprintf("%s\\%s", a.domain, a.username)
	}
	auth, err := ntlmssp.ProcessChallenge(challengeBytes, user, a.password, strings.Contains(user, "\\"))
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(auth)
}
