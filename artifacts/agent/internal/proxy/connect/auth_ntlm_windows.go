//go:build windows

package connectproxy

import (
	"encoding/base64"
	"fmt"

	"github.com/alexbrainman/sspi"
	"github.com/alexbrainman/sspi/negotiate"
)

// ntlmAuth on Windows is implemented via SSPI Negotiate (Kerberos/NTLM).
// When credentials are provided they are passed to SSPI; when empty, the
// current user's credentials are used automatically (SSO).
type ntlmAuth struct {
	domain   string
	username string
	password string
	cred     *sspi.Credentials
	ctx      *negotiate.ClientContext
}

func AuthNTLM(domain, username, password string) ProxyAuthorization {
	return &ntlmAuth{domain: domain, username: username, password: password}
}

func AuthNegotiate(domain, username, password string) ProxyAuthorization {
	return &ntlmAuth{domain: domain, username: username, password: password}
}

func (a *ntlmAuth) Type() string { return "Negotiate" }

func (a *ntlmAuth) acquireCreds() error {
	var err error
	if a.username != "" {
		a.cred, err = negotiate.AcquireUserCredentials(a.domain, a.username, a.password)
	} else {
		a.cred, err = negotiate.AcquireCurrentUserCredentials()
	}
	return err
}

func (a *ntlmAuth) InitialResponse() string {
	if err := a.acquireCreds(); err != nil {
		return ""
	}
	// target SPN is empty; proxy auth does not require a specific SPN
	ctx, token, err := negotiate.NewClientContext(a.cred, "")
	if err != nil {
		return ""
	}
	a.ctx = ctx
	return base64.StdEncoding.EncodeToString(token)
}

func (a *ntlmAuth) ChallengeResponse(challenge string) string {
	if a.ctx == nil {
		return ""
	}
	challengeBytes, err := base64.StdEncoding.DecodeString(challenge)
	if err != nil {
		return fmt.Sprintf("base64 decode error: %v", err)
	}
	_, token, err := a.ctx.Update(challengeBytes)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(token)
}
