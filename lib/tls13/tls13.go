// Package tls13 is a TLS 1.3 client (RFC 8446) in Go for the PS2: X25519
// and P-256 key exchange, AES-GCM and ChaCha20-Poly1305, ECDSA, RSA-PSS and
// Ed25519 server signatures, certificate chains checked with crypto/x509
// against an embedded root bundle. No resumption, early data, client
// certificates or TLS 1.2. It also runs on the host (no cgo), where its
// tests live.
package tls13

import (
	"crypto/rand"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"time"
)

// Config configures a client connection.
type Config struct {
	// ServerName is the host name for SNI and certificate checking.
	ServerName string
	// InsecureSkipVerify accepts any certificate chain.
	InsecureSkipVerify bool
	// RootCAs are the trusted roots; nil means the embedded bundle.
	RootCAs *x509.CertPool
	// Time returns the current time for certificate validity; nil means
	// time.Now (sync the RTC first on the console).
	Time func() time.Time
	// Rand is the randomness source; nil means crypto/rand.
	Rand io.Reader
}

// ConnectionState describes a handshaken connection.
type ConnectionState struct {
	Version          uint16 // 0x0304
	CipherSuite      uint16
	PeerCertificates []*x509.Certificate
}

// Conn is a TLS 1.3 client connection over a net.Conn.
type Conn struct {
	conn   net.Conn
	config *Config
	rand   io.Reader

	handshaken bool
	suite      *cipherSuite
	in, out    *trafficKeys
	peerCerts  []*x509.Certificate
	err        error

	hdr      [5]byte
	rawInput []byte // one record's ciphertext
	hsBuf    []byte // handshake bytes not yet consumed
	appBuf   []byte // decrypted application data not yet read
	eof      bool

	clientRandom, sessionID []byte
	testClientHello         []byte // tests: send exactly this ClientHello
	testKey                 []byte // tests: the X25519 private key
}

var timeNow = time.Now

// Client wraps conn as a TLS client. The handshake runs on the first Read,
// Write or Handshake.
func Client(conn net.Conn, config *Config) *Conn {
	if config == nil {
		config = &Config{}
	}
	c := &Conn{conn: conn, config: config, rand: config.Rand}
	if c.rand == nil {
		c.rand = rand.Reader
	}
	return c
}

// Dial connects to addr ("host:port") over TCP and handshakes; the host
// is the server name unless config names one.
func Dial(addr string, config *Config) (*Conn, error) {
	if config == nil {
		config = &Config{}
	}
	if config.ServerName == "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		cfg := *config
		cfg.ServerName = host
		config = &cfg
	}
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c := Client(raw, config)
	if err := c.Handshake(); err != nil {
		raw.Close()
		return nil, err
	}
	return c, nil
}

// Handshake runs the handshake if it has not run yet.
func (c *Conn) Handshake() error {
	if c.handshaken {
		return nil
	}
	if c.err != nil {
		return c.err
	}
	if err := c.clientHandshake(); err != nil {
		if c.err == nil {
			c.err = err
		}
		return err
	}
	return nil
}

// ConnectionState returns the negotiated parameters (after Handshake).
func (c *Conn) ConnectionState() ConnectionState {
	if !c.handshaken {
		return ConnectionState{}
	}
	return ConnectionState{Version: 0x0304, CipherSuite: c.suite.id, PeerCertificates: c.peerCerts}
}

// Read reads decrypted application data.
func (c *Conn) Read(b []byte) (int, error) {
	if err := c.Handshake(); err != nil {
		return 0, err
	}
	for len(c.appBuf) == 0 {
		if c.eof {
			return 0, io.EOF
		}
		if c.err != nil {
			return 0, c.err
		}
		typ, data, err := c.readRecord()
		if err != nil {
			return 0, err
		}
		switch typ {
		case recordApplicationData:
			c.appBuf = append(c.appBuf[:0], data...)
		case recordAlert:
			if len(data) == 2 && data[1] == alertCloseNotify {
				c.eof = true
				return 0, io.EOF
			}
			if len(data) == 2 {
				c.err = AlertError(data[1])
			} else {
				c.err = errors.New("tls13: malformed alert")
			}
			return 0, c.err
		case recordHandshake:
			if err := c.postHandshake(data); err != nil {
				return 0, err
			}
		case recordChangeCipherSpec:
		default:
			return 0, c.fail(alertUnexpectedMessage, errors.New("tls13: unexpected record"))
		}
	}
	n := copy(b, c.appBuf)
	c.appBuf = c.appBuf[n:]
	return n, nil
}

// postHandshake handles handshake messages after the handshake: key
// updates are applied (and answered), session tickets ignored.
func (c *Conn) postHandshake(data []byte) error {
	c.hsBuf = append(c.hsBuf, data...)
	for len(c.hsBuf) >= 4 {
		n := int(c.hsBuf[1])<<16 | int(c.hsBuf[2])<<8 | int(c.hsBuf[3])
		if len(c.hsBuf) < 4+n {
			return nil
		}
		typ, body := c.hsBuf[0], c.hsBuf[4:4+n]
		switch typ {
		case hsNewSessionTicket:
		case hsKeyUpdate:
			if len(body) != 1 {
				return c.fail(alertDecodeError, errDecode)
			}
			next, err := c.suite.nextKeys(c.in)
			if err != nil {
				return err
			}
			c.in = next
			if body[0] == 1 {
				if err := c.writeRecord(recordHandshake, hsMessage(hsKeyUpdate, []byte{0})); err != nil {
					return err
				}
				if c.out, err = c.suite.nextKeys(c.out); err != nil {
					return err
				}
			}
		default:
			return c.fail(alertUnexpectedMessage, errors.New("tls13: unexpected post-handshake message"))
		}
		c.hsBuf = c.hsBuf[4+n:]
	}
	return nil
}

// Write encrypts and sends application data.
func (c *Conn) Write(b []byte) (int, error) {
	if err := c.Handshake(); err != nil {
		return 0, err
	}
	if c.err != nil {
		return 0, c.err
	}
	n := 0
	for len(b) > 0 {
		m := len(b)
		if m > maxPlaintext {
			m = maxPlaintext
		}
		if err := c.writeRecord(recordApplicationData, b[:m]); err != nil {
			return n, err
		}
		n += m
		b = b[m:]
	}
	return n, nil
}

// Close sends close_notify and closes the connection.
func (c *Conn) Close() error {
	if c.handshaken && c.err == nil {
		c.writeRecord(recordAlert, []byte{1, alertCloseNotify})
	}
	return c.conn.Close()
}

func (c *Conn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *Conn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *Conn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
