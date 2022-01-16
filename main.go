// `tsproxy` proxies a TLS connection in Tailscale to a local TCP or Unix socket.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

var (
	to   = flag.String("to", "", "(Local) address to proxy connections to")
	from = flag.String("from", "aproxy", "Tailscale node name to use")
	help = flag.Bool("help", false, "Display a usage message.")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s:	 \nUsage:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(1)
	}

	net, addr := getAddr(*to)

	l := getListener(*from)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("got connection from %v", conn.RemoteAddr())
		go handleConnection(net, addr, conn)
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

func getListener(name string) net.Listener {
	// Interpolate "name" into the address, so two `tsproxy`s for the same user don't clobber each other.
	userDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dir := path.Join(userDir, fmt.Sprintf("tsproxy-%s", name))
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0700)
	}
	s := tsnet.Server{
		Dir:      dir,
		Hostname: name,
	}
	ln, err := s.Listen("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	return tls.NewListener(ln, &tls.Config{
		GetCertificate: tailscale.GetCertificate,
	})
}

func handleConnection(localNet string, localAddr string, c net.Conn) {
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	remoteAddr := c.RemoteAddr().String()

	if who, err := tailscale.WhoIs(ctx, remoteAddr); err != nil {
		log.Printf("error getting Tailscale whois info: %s", err)
		return
	} else {
		log.Printf("established connection at %s to %s on %s", remoteAddr, who.UserProfile.LoginName, who.Node.Name)
	}

	// Establish a new local connection.
	localConn, err := net.Dial(localNet, localAddr)
	if err != nil {
		log.Printf("could not establish local connection: %s", err)
		return
	}
	defer localConn.Close()

	// Copy local-to-remote in a background thread:
	go io.Copy(localConn, c)

	// In this thread, copy remote-to-local.
	io.Copy(c, localConn)
	// When we exit this scope, both connections are closed.
}
