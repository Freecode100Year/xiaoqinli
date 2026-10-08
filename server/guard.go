package server

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TokenEnv names the environment variable holding the bearer token the HTTP
// modes require when it is set.
const TokenEnv = "XQL_HTTP_TOKEN"

// The HTTP modes used to listen on every interface with no authentication,
// and several of their tools write state that is saved to .xql/ and read by
// every later compile: a strategy tag ending in a newline went into the header
// of every Python file the server produced afterwards. A web page could do the
// same through the browser of anyone running `xql http`, since nothing looked
// at Origin. The guard below is what stands in front of both servers.

// ListenAddr resolves the address argument of `xql http`. A bare ":port"
// means loopback; listening on other interfaces takes an explicit host, such
// as 0.0.0.0:8080.
func ListenAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type guard struct {
	token    string
	readOnly bool // listening beyond loopback with no token
}

func newGuard(addr string) *guard {
	g := &guard{token: os.Getenv(TokenEnv)}
	if g.token == "" && !isLoopbackAddr(addr) {
		g.readOnly = true
		fmt.Fprintf(os.Stderr, "warning: %s is not loopback and %s is unset; "+
			"tools that change compiler state are disabled\n", addr, TokenEnv)
	}
	return g
}

// allowedOrigin accepts requests without an Origin (agents, curl) and those
// from a page served by this machine. Anything else is a browser being used
// against a local server, which the MCP transport spec says to refuse.
func allowedOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	h := u.Hostname()
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func (g *guard) authorized(r *http.Request) bool {
	if g.token == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(g.token)) == 1
}

// wrap checks Origin and the token in front of h. /health stays open so a
// container orchestrator can probe the server.
func (g *guard) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedOrigin(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		if r.URL.Path != "/health" && !g.authorized(r) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (g *guard) mutationAllowed() bool { return g == nil || !g.readOnly }

// serve runs h behind the guard with timeouts, so a client that opens a
// connection and never finishes its headers cannot hold it forever.
func serve(addr string, g *guard, h http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           g.wrap(h),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}
