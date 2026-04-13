package testkit

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type ServerSpec struct {
	Name   string
	FakeIP string
	Zone   string
	Handler dns.Handler
}

type ServerInstance struct {
	Name      string
	FakeIP    string
	Zone      string
	Endpoint  string
	Shutdown  func()
}

func StartServer(t *testing.T, spec ServerSpec) ServerInstance {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	addr := pc.LocalAddr().(*net.UDPAddr)
	_ = pc.Close()

	udpServer := &dns.Server{Addr: addr.String(), Net: "udp", Handler: spec.Handler}
	tcpServer := &dns.Server{Addr: addr.String(), Net: "tcp", Handler: spec.Handler}

	go func() { _ = udpServer.ListenAndServe() }()
	go func() { _ = tcpServer.ListenAndServe() }()

	time.Sleep(50 * time.Millisecond)

	return ServerInstance{
		Name:     spec.Name,
		FakeIP:   spec.FakeIP,
		Zone:     spec.Zone,
		Endpoint: addr.String(),
		Shutdown: func() {
			_ = udpServer.Shutdown()
			_ = tcpServer.Shutdown()
		},
	}
}
