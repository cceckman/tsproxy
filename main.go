// `tsproxy` proxies an HTTP session in Tailscale to a local socket,
// adding authentication headers along the way.
//
// This is a combination of two Tailscale programs:
// - proxy-to-grafana, which acts as an reverse proxy that adds webauth headers
// - nginx-auth, which acts on side-band to nginx to report additional headers
//
// This is a reverse proxy for a single servce, since I don't have an nginx
// setup.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

var (
	to          = flag.String("to", "", "(Local) address to proxy connections to")
	from        = flag.String("from", "", "Tailscale node name to register this proxy as")
	help        = flag.Bool("help", false, "Display a usage message.")
	emailHeader = flag.String("emailHeader", "X-Webauth-Email", "Header indicating the email address associated with the authenticated user")
	netHeader   = flag.String("netHeader", "X-Webauth-Network", "Header indicating the Tailscale network associated with the authenticated user")
	userHeader  = flag.String("userHeader", "X-Webauth-User", "Header indicating the user name (prefix of email address) associated with the authenticated user")
	authKeyPath = flag.String("authKeyPath", "", "If present, path of a file containing a Tailscale auth key. Can be used in place of TS_AUTHKEY.")
	statePath = flag.String("statePath", "", "If present, the directory to store persistent state in (e.g. credentials)")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s:	 \nUsage:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(1)
	}
	if *to == "" || *from == "" {
		fmt.Fprintf(os.Stderr, "Missing -to and/or -from")
		flag.Usage()
		os.Exit(1)
	}

	s := getServer()
	c, err := s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}
	p := getProxy(c)
	l := getListener(s, c)

	log.Printf("tsproxy running at %v, proxying to %v", l.Addr(), *to)
	log.Fatal(http.Serve(l, p))
}

func getListener(s *tsnet.Server, c *tailscale.LocalClient) net.Listener {
	ln, err := s.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	return tls.NewListener(ln, &tls.Config{
		GetCertificate: c.GetCertificate,
	})
}

func getProxy(client *tailscale.LocalClient) *httputil.ReverseProxy {
	url, err := url.Parse(fmt.Sprintf("http://%s", *to))
	if err != nil {
		log.Fatal("invalid target address: ", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	d := proxy.Director
	proxy.Director = func(req *http.Request) {
		d(req)
		authenticateRequest(req, client)
	}
	return proxy
}

func authenticateRequest(req *http.Request, client *tailscale.LocalClient) {
	// Check auth on the inbound request.
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	who, err := client.WhoIs(ctx, req.RemoteAddr)
	if err != nil {
		log.Default().Printf("error getting Tailscale whois info: %s", err)
		return
	}

	req.Header.Add(*emailHeader, who.UserProfile.LoginName)
	req.Header.Add(*userHeader, strings.Split(who.UserProfile.LoginName, "@")[0])
	// nginx-auth proxy notes that on shared nodes, the tailnet won't be known.
	if !who.Node.Hostinfo.ShareeNode() {
		_, tailnet, ok := strings.Cut(who.Node.Name, who.Node.ComputedName+".")
		if !ok {
			log.Printf("can't extract tailnet name from hostname %q", who.Node.Name)
		}
		tailnet = strings.TrimSuffix(tailnet, ".beta.tailscale.net")
		req.Header.Add(*netHeader, tailnet)
	}
}

// Resolve the provided address to to a network-and-address, to connect properly.
func getAddr(to string) (net, addr string) {
	if path, err := filepath.Abs(to); err == nil {
		return "unix", path
	}
	// Assume TCP, since we're doing TLS.
	return "tcp", to
}

func getStateDir() string {
	if *statePath != "" {
		return *statePath
	}
	// Use $STATE_DIRECTORY as specified by
	// https://www.freedesktop.org/software/systemd/man/systemd.exec.html -
	// should work for user or system units.
	statePath := os.Getenv("STATE_DIRECTORY")
	// Extend with this particular instance
	return path.Join(statePath, fmt.Sprintf("tsproxy-%s", *from))
}

func getServer() *tsnet.Server {
	dir := getStateDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0700)
	}

	authKey, err := fs.ReadFile(os.DirFS(""), *authKeyPath)
	if *authKeyPath != "" && err != nil {
		log.Printf("could not read TS auth key from %s: %v; continuing", *authKeyPath, err)
	}

	s := &tsnet.Server{
		Dir:      dir,
		Hostname: *from,
		AuthKey:  string(authKey),
	}

	if err := s.Start(); err != nil {
		log.Fatal("error starting Tailscale server: ", err)
	}
	return s
}
