package csta

import (
	"crypto/tls"
	"net"
	"strconv"
	"time"
)

// Default time intervals used.
var (
	DefaultConnectionTimeout = time.Second * 5  // timeout for server connection.
	DefaultKeepAliveDuration = time.Second * 30 // period for sending keep-alive messages.
)

// Addr describes the address and parameters for connecting to the MX server via CSTA protocol.
type Addr struct {
	Host           string        // server address
	Port           int           `yaml:",omitempty"`               // port
	Secure         bool          `yaml:",omitempty"`               // use secure connection
	SkipVerify     bool          `yaml:"skipVerify,omitempty"`     // do not verify certificate validity
	Timeout        time.Duration `yaml:",omitempty"`               // connection timeout
	ReconnectDelay time.Duration `yaml:"reconnectDelay,omitempty"` // delay time between reconnecting to the server
	MaxError       int           `yaml:"maxError,omitempty"`       // maximum allowable number of errors
}

// FullAddr returns the full server address, including the port.
func (a *Addr) FullAddr() string {
	port := a.Port // server port
	if port == 0 {
		if a.Secure {
			port = 7778 // default port for secure connection
		} else {
			port = 7777 // default port for non-secure connection
		}
	}
	host := a.Host // server address
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, strconv.Itoa(port)) // full server address, including port
}

// Dial establishes and returns a connection to the server.
func (a *Addr) Dial() (net.Conn, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = DefaultConnectionTimeout
	}
	dialer := &net.Dialer{ // connection establisher
		Timeout:   timeout,          // maximum connection wait time
		KeepAlive: time.Second * 10, // connection maintenance interval
	}
	addr := a.FullAddr() // get full address, including port
	if a.Secure {        // establish secure connection
		// do not check certificate validity if specified in settings
		return tls.DialWithDialer(dialer, "tcp", addr,
			&tls.Config{InsecureSkipVerify: a.SkipVerify})
	}
	return dialer.Dial("tcp", addr) // establish non-secure connection
}
