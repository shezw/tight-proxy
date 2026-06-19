package relay

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/shezw/tight-proxy/internal/config"
)

func TestRelayForwardsTCP(t *testing.T) {
	exitListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer exitListener.Close()

	go func() {
		conn, err := exitListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(conn, "echo:%s", line)
	}()

	exitHost, exitPortText, err := net.SplitHostPort(exitListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	exitPort, err := strconv.Atoi(exitPortText)
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Relay{
		Enabled: true,
		Rules: []config.RelayRule{
			{
				Enabled: true,
				Entry:   config.Listen{Host: "127.0.0.1", Port: 0},
				Exit:    config.Listen{Host: exitHost, Port: exitPort},
			},
		},
	})
	addrs, err := server.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	if len(addrs) != 1 {
		t.Fatalf("expected 1 relay listener, got %d", len(addrs))
	}

	conn, err := net.DialTimeout("tcp", addrs[0].String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if reply != "echo:ping\n" {
		t.Fatalf("unexpected reply %q", reply)
	}
}
