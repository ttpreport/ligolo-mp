// thanks https://github.com/wrouesnel/go.connect-proxy-scheme
package connectproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"golang.org/x/net/proxy"

	"net"
	"net/http"
	"net/url"
	"strings"
)

// New constructs an HttpConnectTunnel to be used a net.Dial command.
// The first parameter is a proxy URL, for example https://foo.example.com:9090 will use foo.example.com as proxy on
// port 9090 using TLS for connectivity.
func New(proxyUrl *url.URL, dialer proxy.Dialer, timeout time.Duration, tlsConfig *tls.Config) (*HttpConnectTunnel, error) {
	scheme := proxyUrl.Scheme
	isNTLM := scheme == "ntlm"

	// ntlm:// tunnels over plain HTTP CONNECT; treat it as http for transport.
	if isNTLM {
		scheme = "http"
	}

	t := &HttpConnectTunnel{
		timeout:      timeout,
		parentDialer: dialer,
		proxyScheme:  scheme,
		proxyHost:    proxyUrl.Hostname(),
		proxyPort:    proxyUrl.Port(),
		proxyPath:    proxyUrl.Path,
	}

	if tlsConfig != nil {
		t.isTls = true
		t.tlsConfig = tlsConfig
	}

	if t.proxyPort == "" {
		if t.proxyScheme == "https" {
			t.proxyPort = "443"
		} else {
			t.proxyPort = "8080"
		}
	}

	rawUser := proxyUrl.User.Username()
	pass, _ := proxyUrl.User.Password()

	if isNTLM {
		// ntlm:// — explicit NTLM/Negotiate auth.
		// Credentials optional: empty = SSO with current user (Windows only).
		domain, user := splitDomainUser(rawUser)
		t.auth = AuthNegotiate(domain, user, pass)
	} else if rawUser != "" {
		// http(s):// with credentials — Basic auth; explicit always wins.
		// Store them also for SSO fallback in case the proxy rejects Basic.
		domain, user := splitDomainUser(rawUser)
		if domain != "" {
			// DOMAIN\user format → treat as NTLM explicit
			t.auth = AuthNTLM(domain, user, pass)
		} else {
			t.auth = AuthBasic(rawUser, pass)
		}
	}
	// http(s):// with no credentials: t.auth stays nil; 407 auto-detection
	// via selectAuthFromResponse will attempt SSO if the proxy requests it.

	return t, nil
}

// splitDomainUser splits "DOMAIN\user" or "DOMAIN/user" into (domain, user).
// Returns ("", raw) if no domain separator is found.
func splitDomainUser(raw string) (domain, user string) {
	for _, sep := range []string{"\\", "/"} {
		if idx := strings.Index(raw, sep); idx >= 0 {
			return raw[:idx], raw[idx+1:]
		}
	}
	return "", raw
}

func HttpHandler(timeout time.Duration) func(proxyUrl *url.URL, dialer proxy.Dialer) (proxy.Dialer, error) {
	return func(proxyUrl *url.URL, dialer proxy.Dialer) (proxy.Dialer, error) {
		return New(proxyUrl, dialer, timeout, nil)
	}
}

func HttpsHandler(timeout time.Duration, tlsConfig *tls.Config) func(proxyUrl *url.URL, dialer proxy.Dialer) (proxy.Dialer, error) {
	return func(proxyUrl *url.URL, dialer proxy.Dialer) (proxy.Dialer, error) {
		return New(proxyUrl, dialer, timeout, tlsConfig)
	}
}

var _ = proxy.Dialer(HttpConnectTunnel{})
var _ = proxy.ContextDialer(HttpConnectTunnel{})

// HttpConnectTunnel represents a configured HTTP Connect Tunnel dialer.
type HttpConnectTunnel struct {
	timeout      time.Duration
	isTls        bool
	tlsConfig    *tls.Config
	parentDialer proxy.Dialer
	proxyScheme  string
	proxyHost    string
	proxyPort    string
	proxyPath    string
	auth         ProxyAuthorization
	// sso* hold credentials for auto-detected SSO (empty = use current user on Windows)
	ssoDomain string
	ssoUser   string
	ssoPass   string
}

func (t HttpConnectTunnel) dialProxy(ctx context.Context) (net.Conn, error) {
	var conn net.Conn
	var err error

	if f, ok := t.parentDialer.(proxy.ContextDialer); ok {
		conn, err = f.DialContext(ctx, "tcp", net.JoinHostPort(t.proxyHost, t.proxyPort))
	} else {
		conn, err = dialContext(ctx, t.parentDialer, "tcp", net.JoinHostPort(t.proxyHost, t.proxyPort))
	}

	if t.isTls {
		conn = tls.Client(conn, t.tlsConfig)
	}

	return conn, err
}

func (t HttpConnectTunnel) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("network type '%v' unsupported (only 'tcp')", network)
	}
	conn, err := t.dialProxy(ctx)
	if err != nil {
		return nil, fmt.Errorf("http_tunnel: failed dialing to proxy: %v", err)
	}
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}

	auth := t.auth
	if auth != nil {
		if initial := auth.InitialResponse(); initial != "" {
			req.Header.Set(hdrProxyAuthResp, auth.Type()+" "+initial)
		}
	}

	resp, err := t.doRoundtrip(conn, req)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// On 407, auto-detect auth scheme if none was pre-configured.
	if resp.StatusCode == http.StatusProxyAuthRequired && auth == nil {
		auth = selectAuthFromResponse(resp, t.ssoDomain, t.ssoUser, t.ssoPass)
		if auth != nil {
			if initial := auth.InitialResponse(); initial != "" {
				req.Header.Set(hdrProxyAuthResp, auth.Type()+" "+initial)
				resp, err = t.doRoundtrip(conn, req)
				if err != nil {
					conn.Close()
					return nil, err
				}
			}
		}
	}

	// Handle challenge-response round (NTLM type 2 → type 3).
	if resp.StatusCode == http.StatusProxyAuthRequired && auth != nil {
		responseHdr, err := performChallengeResponse(auth, resp)
		if err != nil {
			conn.Close()
			return nil, err
		}
		req.Header.Set(hdrProxyAuthResp, auth.Type()+" "+responseHdr)
		resp, err = t.doRoundtrip(conn, req)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	if resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("http_tunnel: failed proxying %d: %s", resp.StatusCode, resp.Status)
	}

	return conn, nil
}

// Dial is an implementation of net.Dialer, and returns a TCP connection handle to the host that HTTP CONNECT reached.
func (t HttpConnectTunnel) Dial(network string, address string) (net.Conn, error) {
	ctx, _ := context.WithTimeout(context.Background(), t.timeout)
	return t.DialContext(ctx, network, address)
}

func (t HttpConnectTunnel) doRoundtrip(conn net.Conn, req *http.Request) (*http.Response, error) {
	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("http_tunnel: failed writing request: %v", err)
	}
	// Doesn't matter, discard this bufio.
	br := bufio.NewReader(conn)
	return http.ReadResponse(br, req)

}

// performChallengeResponse extracts the server's token from a 407 response
// and returns the client's response token. The server token may be absent
// (e.g. first NTLM 407 that only advertises the scheme name).
func performChallengeResponse(auth ProxyAuthorization, resp *http.Response) (string, error) {
	// The proxy may send multiple Proxy-Authenticate headers; find ours.
	challenge := ""
	for _, hdr := range resp.Header[hdrProxyAuthReq] {
		if strings.EqualFold(strings.SplitN(hdr, " ", 2)[0], auth.Type()) {
			parts := strings.SplitN(hdr, " ", 2)
			if len(parts) == 2 {
				challenge = parts[1]
			}
			break
		}
	}
	token := auth.ChallengeResponse(challenge)
	return token, nil
}

// selectAuthFromResponse inspects the 407 Proxy-Authenticate headers and
// returns the best available ProxyAuthorization: Negotiate > NTLM.
// domain/user/pass are the SSO credentials (empty = current user on Windows).
func selectAuthFromResponse(resp *http.Response, domain, user, pass string) ProxyAuthorization {
	var hasNegotiate, hasNTLM bool
	for _, hdr := range resp.Header[hdrProxyAuthReq] {
		scheme := strings.ToLower(strings.SplitN(hdr, " ", 2)[0])
		switch scheme {
		case "negotiate":
			hasNegotiate = true
		case "ntlm":
			hasNTLM = true
		}
	}
	switch {
	case hasNegotiate:
		return AuthNegotiate(domain, user, pass)
	case hasNTLM:
		return AuthNTLM(domain, user, pass)
	}
	return nil
}

// WARNING: this can leak a goroutine for as long as the underlying Dialer implementation takes to timeout
// A Conn returned from a successful Dial after the context has been cancelled will be immediately closed.
func dialContext(ctx context.Context, d proxy.Dialer, network, address string) (net.Conn, error) {
	var (
		conn net.Conn
		done = make(chan struct{}, 1)
		err  error
	)
	go func() {
		conn, err = d.Dial(network, address)
		close(done)
		if conn != nil && ctx.Err() != nil {
			conn.Close()
		}
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-done:
	}
	return conn, err
}
