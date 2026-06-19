package proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/shezw/tight-proxy/internal/config"
)

func dialTarget(address string, upstream *config.Upstream) (net.Conn, error) {
	if upstream == nil {
		return net.DialTimeout("tcp", address, 15*time.Second)
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(upstream.Type) {
	case "socks", "socks5":
		return dialSOCKS5(host, port, *upstream)
	case "http", "https", "ftp":
		return dialHTTPProxy(address, *upstream)
	default:
		return nil, fmt.Errorf("unsupported upstream proxy type: %s", upstream.Type)
	}
}

func dialHTTPProxy(address string, upstream config.Upstream) (net.Conn, error) {
	proxyAddr := net.JoinHostPort(upstream.Host, strconv.Itoa(upstream.Port))
	raw, err := net.DialTimeout("tcp", proxyAddr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	conn := raw
	if strings.EqualFold(upstream.Type, "https") {
		tlsConn := tls.Client(raw, &tls.Config{ServerName: upstream.Host})
		if err := tlsConn.Handshake(); err != nil {
			_ = raw.Close()
			return nil, err
		}
		conn = tlsConn
	}
	auth := proxyAuthHeader(upstream)
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n%sConnection: keep-alive\r\n\r\n", address, address, auth)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !strings.Contains(line, " 200 ") {
		_ = conn.Close()
		return nil, fmt.Errorf("upstream CONNECT failed: %s", strings.TrimSpace(line))
	}
	for {
		line, err = br.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if line == "\r\n" {
			break
		}
	}
	if br.Buffered() > 0 {
		conn = &bufferedConn{Conn: conn, reader: br}
	}
	return conn, nil
}

func dialSOCKS5(host string, port int, upstream config.Upstream) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(upstream.Host, strconv.Itoa(upstream.Port)), 15*time.Second)
	if err != nil {
		return nil, err
	}
	needsAuth := upstream.Username != "" || upstream.Password != ""
	if needsAuth {
		_, err = conn.Write([]byte{0x05, 0x02, 0x00, 0x02})
	} else {
		_, err = conn.Write([]byte{0x05, 0x01, 0x00})
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if reply[0] != 0x05 || reply[1] == 0xff {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 authentication method rejected")
	}
	if reply[1] == 0x02 {
		user := []byte(upstream.Username)
		pass := []byte(upstream.Password)
		if len(user) > 255 || len(pass) > 255 {
			_ = conn.Close()
			return nil, fmt.Errorf("SOCKS5 credentials are too long")
		}
		packet := []byte{0x01, byte(len(user))}
		packet = append(packet, user...)
		packet = append(packet, byte(len(pass)))
		packet = append(packet, pass...)
		if _, err := conn.Write(packet); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if _, err := io.ReadFull(conn, reply); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if reply[1] != 0x00 {
			_ = conn.Close()
			return nil, fmt.Errorf("SOCKS5 authentication failed")
		}
	}
	req := []byte{0x05, 0x01, 0x00}
	req = append(req, encodeSOCKSAddress(host)...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	req = append(req, portBytes...)
	if _, err := conn.Write(req); err != nil {
		_ = conn.Close()
		return nil, err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if header[1] != 0x00 {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 connect failed with code %d", header[1])
	}
	switch header[3] {
	case 0x01:
		_, err = io.CopyN(io.Discard, conn, 4)
	case 0x03:
		size := make([]byte, 1)
		if _, err = io.ReadFull(conn, size); err == nil {
			_, err = io.CopyN(io.Discard, conn, int64(size[0]))
		}
	case 0x04:
		_, err = io.CopyN(io.Discard, conn, 16)
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := io.CopyN(io.Discard, conn, 2); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func encodeSOCKSAddress(host string) []byte {
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return append([]byte{0x01}, v4...)
		}
		return append([]byte{0x04}, ip.To16()...)
	}
	out := []byte{0x03, byte(len(host))}
	return append(out, []byte(host)...)
}

func proxyAuthHeader(upstream config.Upstream) string {
	if upstream.Username == "" && upstream.Password == "" {
		return ""
	}
	token := base64.StdEncoding.EncodeToString([]byte(upstream.Username + ":" + upstream.Password))
	return "Proxy-Authorization: Basic " + token + "\r\n"
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
