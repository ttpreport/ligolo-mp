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
	t := &HttpConnectTunnel{
		timeout:      timeout, // not sure this actually works as intended
		parentDialer: dialer,
		proxyScheme:  proxyUrl.Scheme,
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

	u := proxyUrl.User.Username()
	if u != "" {
		p, _ := proxyUrl.User.Password()
		t.auth = AuthBasic(u, p)
	}

	return t, nil
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
		Host:   address, // This is weird
		Header: make(http.Header),
	}
	if t.auth != nil && t.auth.InitialResponse() != "" {
		req.Header.Set(hdrProxyAuthResp, t.auth.Type()+" "+t.auth.InitialResponse())
	}
	resp, err := t.doRoundtrip(conn, req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// Retry request with auth, if available.
	if resp.StatusCode == http.StatusProxyAuthRequired && t.auth != nil {
		responseHdr, err := t.performAuthChallengeResponse(resp)
		if err != nil {
			conn.Close()
			return nil, err
		}
		req.Header.Set(hdrProxyAuthResp, t.auth.Type()+" "+responseHdr)
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

func (t HttpConnectTunnel) performAuthChallengeResponse(resp *http.Response) (string, error) {
	respAuthHdr := resp.Header.Get(hdrProxyAuthReq)
	if !strings.Contains(respAuthHdr, t.auth.Type()+" ") {
		return "", fmt.Errorf("http_tunnel: expected '%v' Proxy authentication, got: '%v'", t.auth.Type(), respAuthHdr)
	}
	splits := strings.SplitN(respAuthHdr, " ", 2)
	challenge := splits[1]
	return t.auth.ChallengeResponse(challenge), nil
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
