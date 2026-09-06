package tls13

import (
	"bytes"
	"crypto/ecdh"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
)

// Handshake message types.
const (
	hsClientHello         = 1
	hsServerHello         = 2
	hsNewSessionTicket    = 4
	hsEncryptedExtensions = 8
	hsCertificate         = 11
	hsCertificateVerify   = 15
	hsFinished            = 20
	hsKeyUpdate           = 24
	hsMessageHash         = 254
)

// Extensions.
const (
	extServerName          = 0
	extSupportedGroups     = 10
	extSignatureAlgorithms = 13
	extSupportedVersions   = 43
	extCookie              = 44
	extPSKModes            = 45
	extKeyShare            = 51
)

// Named groups and signature schemes (RFC 8446 section 4.2).
const (
	groupX25519    uint16 = 0x001d
	groupSecp256r1 uint16 = 0x0017

	sigECDSAP256SHA256 uint16 = 0x0403
	sigECDSAP384SHA384 uint16 = 0x0503
	sigECDSAP521SHA512 uint16 = 0x0603
	sigRSAPSSSHA256    uint16 = 0x0804
	sigRSAPSSSHA384    uint16 = 0x0805
	sigRSAPSSSHA512    uint16 = 0x0806
	sigEd25519         uint16 = 0x0807
)

var helloRetryRequestRandom = []byte{
	0xCF, 0x21, 0xAD, 0x74, 0xE5, 0x9A, 0x61, 0x11, 0xBE, 0x1D, 0x8C, 0x02, 0x1E, 0x65, 0xB8, 0x91,
	0xC2, 0xA2, 0x11, 0x16, 0x7A, 0xBB, 0x8C, 0x5E, 0x07, 0x9E, 0x09, 0xE2, 0xC8, 0xA8, 0x33, 0x9C,
}

// A small builder for TLS vectors.
type builder struct{ b []byte }

func (w *builder) u8(v byte)      { w.b = append(w.b, v) }
func (w *builder) u16(v uint16)   { w.b = append(w.b, byte(v>>8), byte(v)) }
func (w *builder) bytes(p []byte) { w.b = append(w.b, p...) }

// vec appends a length-prefixed vector produced by f, with an n-byte length.
func (w *builder) vec(n int, f func(w *builder)) {
	start := len(w.b)
	w.b = append(w.b, make([]byte, n)...)
	f(w)
	l := len(w.b) - start - n
	for i := 0; i < n; i++ {
		w.b[start+i] = byte(l >> (8 * (n - 1 - i)))
	}
}

// A reader for TLS vectors.
type reader struct{ b []byte }

func (r *reader) u8() (byte, bool) {
	if len(r.b) < 1 {
		return 0, false
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v, true
}

func (r *reader) u16() (uint16, bool) {
	if len(r.b) < 2 {
		return 0, false
	}
	v := binary.BigEndian.Uint16(r.b)
	r.b = r.b[2:]
	return v, true
}

func (r *reader) take(n int) ([]byte, bool) {
	if len(r.b) < n {
		return nil, false
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v, true
}

// vec reads an n-byte length prefix and that many bytes.
func (r *reader) vec(n int) ([]byte, bool) {
	l, ok := r.take(n)
	if !ok {
		return nil, false
	}
	length := 0
	for _, x := range l {
		length = length<<8 | int(x)
	}
	return r.take(length)
}

var errDecode = errors.New("tls13: malformed handshake message")

func decodeErr(what string) error { return fmt.Errorf("tls13: malformed %s", what) }

// keyShare is one of the client's ephemeral keys.
type keyShare struct {
	group uint16
	key   *ecdh.PrivateKey
}

func (c *Conn) makeKeyShares(groups []uint16) ([]keyShare, error) {
	var shares []keyShare
	for _, g := range groups {
		var curve ecdh.Curve
		switch g {
		case groupX25519:
			curve = ecdh.X25519()
		case groupSecp256r1:
			curve = ecdh.P256()
		default:
			continue
		}
		var k *ecdh.PrivateKey
		var err error
		if c.testKey != nil && g == groupX25519 {
			k, err = curve.NewPrivateKey(c.testKey)
		} else {
			k, err = curve.GenerateKey(c.rand)
		}
		if err != nil {
			return nil, err
		}
		shares = append(shares, keyShare{g, k})
	}
	return shares, nil
}

// clientHello builds the ClientHello for the key shares (and cookie after
// a HelloRetryRequest).
func (c *Conn) clientHello(shares []keyShare, cookie []byte) []byte {
	w := &builder{}
	w.u8(hsClientHello)
	w.vec(3, func(w *builder) {
		w.u16(0x0303)
		w.bytes(c.clientRandom)
		w.vec(1, func(w *builder) { w.bytes(c.sessionID) })
		w.vec(2, func(w *builder) {
			for _, s := range cipherSuites {
				w.u16(s.id)
			}
		})
		w.vec(1, func(w *builder) { w.u8(0) })
		w.vec(2, func(w *builder) {
			if c.config.ServerName != "" {
				w.u16(extServerName)
				w.vec(2, func(w *builder) {
					w.vec(2, func(w *builder) {
						w.u8(0)
						w.vec(2, func(w *builder) { w.bytes([]byte(c.config.ServerName)) })
					})
				})
			}
			w.u16(extSupportedGroups)
			w.vec(2, func(w *builder) {
				w.vec(2, func(w *builder) {
					w.u16(groupX25519)
					w.u16(groupSecp256r1)
				})
			})
			w.u16(extSignatureAlgorithms)
			w.vec(2, func(w *builder) {
				w.vec(2, func(w *builder) {
					for _, s := range []uint16{sigECDSAP256SHA256, sigECDSAP384SHA384, sigECDSAP521SHA512,
						sigRSAPSSSHA256, sigRSAPSSSHA384, sigRSAPSSSHA512, sigEd25519} {
						w.u16(s)
					}
				})
			})
			w.u16(extSupportedVersions)
			w.vec(2, func(w *builder) {
				w.vec(1, func(w *builder) { w.u16(0x0304) })
			})
			w.u16(extPSKModes)
			w.vec(2, func(w *builder) {
				w.vec(1, func(w *builder) { w.u8(1) })
			})
			if cookie != nil {
				w.u16(extCookie)
				w.vec(2, func(w *builder) {
					w.vec(2, func(w *builder) { w.bytes(cookie) })
				})
			}
			w.u16(extKeyShare)
			w.vec(2, func(w *builder) {
				w.vec(2, func(w *builder) {
					for _, s := range shares {
						w.u16(s.group)
						w.vec(2, func(w *builder) { w.bytes(s.key.PublicKey().Bytes()) })
					}
				})
			})
		})
	})
	return w.b
}

// serverHello is what the client needs from a ServerHello.
type serverHello struct {
	retry     bool // HelloRetryRequest
	suite     *cipherSuite
	group     uint16
	peerKey   []byte
	cookie    []byte
	sessionID []byte
}

func parseServerHello(body []byte) (*serverHello, error) {
	r := &reader{body}
	sh := &serverHello{}
	if v, ok := r.u16(); !ok || v != 0x0303 {
		return nil, decodeErr("ServerHello version")
	}
	random, ok := r.take(32)
	if !ok {
		return nil, decodeErr("ServerHello random")
	}
	sh.retry = bytes.Equal(random, helloRetryRequestRandom)
	if sh.sessionID, ok = r.vec(1); !ok {
		return nil, decodeErr("ServerHello session id")
	}
	id, ok := r.u16()
	if !ok {
		return nil, decodeErr("ServerHello cipher suite")
	}
	if sh.suite = suiteByID(id); sh.suite == nil {
		return nil, fmt.Errorf("tls13: server chose unknown cipher suite %#x", id)
	}
	if comp, ok := r.u8(); !ok || comp != 0 {
		return nil, decodeErr("ServerHello compression")
	}
	exts, ok := r.vec(2)
	if !ok {
		return nil, decodeErr("ServerHello extensions")
	}
	er := &reader{exts}
	sawVersion := false
	for len(er.b) > 0 {
		typ, ok := er.u16()
		if !ok {
			return nil, decodeErr("ServerHello extension type")
		}
		data, ok := er.vec(2)
		if !ok {
			return nil, decodeErr("ServerHello extension data")
		}
		dr := &reader{data}
		switch typ {
		case extSupportedVersions:
			v, ok := dr.u16()
			if !ok || v != 0x0304 {
				return nil, errors.New("tls13: server did not select TLS 1.3")
			}
			sawVersion = true
		case extKeyShare:
			if sh.group, ok = dr.u16(); !ok {
				return nil, decodeErr("key share group")
			}
			if !sh.retry {
				if sh.peerKey, ok = dr.vec(2); !ok {
					return nil, decodeErr("key share key")
				}
			}
		case extCookie:
			if sh.cookie, ok = dr.vec(2); !ok {
				return nil, decodeErr("cookie")
			}
		}
	}
	if !sawVersion {
		return nil, errors.New("tls13: server did not select TLS 1.3")
	}
	return sh, nil
}

// certificate holds the server's chain as sent.
func parseCertificate(body []byte) ([]*x509.Certificate, error) {
	r := &reader{body}
	if _, ok := r.vec(1); !ok { // certificate_request_context
		return nil, decodeErr("Certificate context")
	}
	list, ok := r.vec(3)
	if !ok {
		return nil, decodeErr("Certificate list")
	}
	lr := &reader{list}
	var certs []*x509.Certificate
	for len(lr.b) > 0 {
		der, ok := lr.vec(3)
		if !ok {
			return nil, decodeErr("Certificate entry")
		}
		if _, ok := lr.vec(2); !ok { // extensions
			return nil, decodeErr("Certificate extensions")
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("tls13: bad certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, errors.New("tls13: server sent no certificate")
	}
	return certs, nil
}

// verifySignature checks a CertificateVerify signature over the transcript.
func verifySignature(leaf *x509.Certificate, scheme uint16, transcript, sig []byte) error {
	var algo x509.SignatureAlgorithm
	switch scheme {
	case sigECDSAP256SHA256:
		algo = x509.ECDSAWithSHA256
	case sigECDSAP384SHA384:
		algo = x509.ECDSAWithSHA384
	case sigECDSAP521SHA512:
		algo = x509.ECDSAWithSHA512
	case sigRSAPSSSHA256:
		algo = x509.SHA256WithRSAPSS
	case sigRSAPSSSHA384:
		algo = x509.SHA384WithRSAPSS
	case sigRSAPSSSHA512:
		algo = x509.SHA512WithRSAPSS
	case sigEd25519:
		algo = x509.PureEd25519
	default:
		return fmt.Errorf("tls13: unsupported signature scheme %#x", scheme)
	}
	// The signed content: 64 spaces, the context string, a zero, the hash.
	content := make([]byte, 0, 64+33+1+len(transcript))
	for i := 0; i < 64; i++ {
		content = append(content, ' ')
	}
	content = append(content, "TLS 1.3, server CertificateVerify"...)
	content = append(content, 0)
	content = append(content, transcript...)
	return leaf.CheckSignature(algo, content, sig)
}

// transcript accumulates the handshake messages for the transcript hash.
type transcript struct {
	h    hash.Hash
	msgs []byte // until the suite is known
}

func (t *transcript) add(msg []byte) {
	if t.h != nil {
		t.h.Write(msg)
	} else {
		t.msgs = append(t.msgs, msg...)
	}
}

func (t *transcript) start(s *cipherSuite) {
	t.h = s.newHash()
	t.h.Write(t.msgs)
	t.msgs = nil
}

func (t *transcript) sum() []byte {
	return t.h.Sum(nil)
}

// clientHandshake runs the client side of a TLS 1.3 handshake.
func (c *Conn) clientHandshake() error {
	if c.clientRandom == nil {
		c.clientRandom = make([]byte, 32)
		if _, err := io.ReadFull(c.rand, c.clientRandom); err != nil {
			return err
		}
	}
	if c.sessionID == nil {
		c.sessionID = make([]byte, 32)
		if _, err := io.ReadFull(c.rand, c.sessionID); err != nil {
			return err
		}
	}
	shares, err := c.makeKeyShares([]uint16{groupX25519, groupSecp256r1})
	if err != nil {
		return err
	}
	tr := &transcript{}
	hello := c.clientHello(shares, nil)
	if c.testClientHello != nil {
		hello = c.testClientHello
	}
	if err := c.writeRecord(recordHandshake, hello); err != nil {
		return err
	}
	tr.add(hello)

	typ, body, err := c.readHandshake()
	if err != nil {
		return err
	}
	if typ != hsServerHello {
		return c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: expected ServerHello, got %d", typ))
	}
	sh, err := parseServerHello(body)
	if err != nil {
		return c.fail(alertIllegalParameter, err)
	}
	suite := sh.suite
	if sh.retry {
		// HelloRetryRequest: the transcript restarts with a hash of the
		// first ClientHello; send a second one for the requested group.
		tr.start(suite)
		first := tr.sum()
		tr = &transcript{}
		tr.start(suite)
		tr.add(append([]byte{hsMessageHash, 0, 0, byte(len(first))}, first...))
		tr.add(append([]byte{hsServerHello, 0, 0, byte(len(body))}, body...))
		var retryShares []keyShare
		for _, s := range shares {
			if s.group == sh.group {
				retryShares = append(retryShares, s)
			}
		}
		if len(retryShares) == 0 {
			return c.fail(alertIllegalParameter, fmt.Errorf("tls13: server asked for group %#x", sh.group))
		}
		hello = c.clientHello(retryShares, sh.cookie)
		if err := c.writeRecord(recordHandshake, hello); err != nil {
			return err
		}
		tr.add(hello)
		if typ, body, err = c.readHandshake(); err != nil {
			return err
		}
		if typ != hsServerHello {
			return c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: expected ServerHello, got %d", typ))
		}
		if sh, err = parseServerHello(body); err != nil {
			return c.fail(alertIllegalParameter, err)
		}
		if sh.retry || sh.suite != suite {
			return c.fail(alertIllegalParameter, errors.New("tls13: bad second ServerHello"))
		}
	} else {
		tr.start(suite)
	}
	if !bytes.Equal(sh.sessionID, c.sessionID) {
		return c.fail(alertIllegalParameter, errors.New("tls13: session id mismatch"))
	}
	tr.add(append([]byte{hsServerHello, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...))

	// Key exchange and the handshake secrets.
	var shared []byte
	for _, s := range shares {
		if s.group == sh.group {
			peer, err := s.key.Curve().NewPublicKey(sh.peerKey)
			if err != nil {
				return c.fail(alertIllegalParameter, err)
			}
			if shared, err = s.key.ECDH(peer); err != nil {
				return c.fail(alertIllegalParameter, err)
			}
		}
	}
	if shared == nil {
		return c.fail(alertIllegalParameter, fmt.Errorf("tls13: server key share for group %#x", sh.group))
	}
	early := suite.extract(nil, nil)
	handshakeSecret := suite.extract(suite.deriveSecret(early, "derived", suite.hashEmpty()), shared)
	th := tr.sum()
	clientHS := suite.deriveSecret(handshakeSecret, "c hs traffic", th)
	serverHS := suite.deriveSecret(handshakeSecret, "s hs traffic", th)
	if c.in, err = suite.trafficKeys(serverHS); err != nil {
		return err
	}
	if c.out, err = suite.trafficKeys(clientHS); err != nil {
		return err
	}
	master := suite.extract(suite.deriveSecret(handshakeSecret, "derived", suite.hashEmpty()), nil)

	// The server's encrypted flight.
	if typ, body, err = c.readHandshake(); err != nil {
		return fmt.Errorf("%w (suite %#x, group %#x)", err, suite.id, sh.group)
	}
	if typ != hsEncryptedExtensions {
		return c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: expected EncryptedExtensions, got %d", typ))
	}
	tr.add(hsMessage(typ, body))
	if typ, body, err = c.readHandshake(); err != nil {
		return err
	}
	if typ != hsCertificate {
		return c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: expected Certificate, got %d", typ))
	}
	certs, err := parseCertificate(body)
	if err != nil {
		return c.fail(alertBadCertificate, err)
	}
	tr.add(hsMessage(typ, body))
	if err := c.verifyChain(certs); err != nil {
		return c.fail(alertBadCertificate, err)
	}
	if typ, body, err = c.readHandshake(); err != nil {
		return err
	}
	if typ != hsCertificateVerify {
		return c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: expected CertificateVerify, got %d", typ))
	}
	r := &reader{body}
	scheme, ok := r.u16()
	sig, ok2 := r.vec(2)
	if !ok || !ok2 {
		return c.fail(alertDecodeError, decodeErr("CertificateVerify"))
	}
	if err := verifySignature(certs[0], scheme, tr.sum(), sig); err != nil {
		return c.fail(alertDecryptError, fmt.Errorf("tls13: certificate verify: %w", err))
	}
	tr.add(hsMessage(typ, body))
	if typ, body, err = c.readHandshake(); err != nil {
		return err
	}
	if typ != hsFinished {
		return c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: expected Finished, got %d", typ))
	}
	if !bytes.Equal(body, suite.finishedMAC(serverHS, tr.sum())) {
		return c.fail(alertDecryptError, errors.New("tls13: server Finished does not verify"))
	}
	tr.add(hsMessage(typ, body))

	// Application secrets, then our Finished.
	th = tr.sum()
	clientAP := suite.deriveSecret(master, "c ap traffic", th)
	serverAP := suite.deriveSecret(master, "s ap traffic", th)
	fin := hsMessage(hsFinished, suite.finishedMAC(clientHS, th))
	if err := c.writeRecord(recordHandshake, fin); err != nil {
		return err
	}
	if c.in, err = suite.trafficKeys(serverAP); err != nil {
		return err
	}
	if c.out, err = suite.trafficKeys(clientAP); err != nil {
		return err
	}
	c.suite = suite
	c.peerCerts = certs
	c.handshaken = true
	return nil
}

// hsMessage frames a handshake message body with its type and length.
func hsMessage(typ byte, body []byte) []byte {
	return append([]byte{typ, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)
}

func (s *cipherSuite) hashEmpty() []byte {
	h := s.newHash()
	return h.Sum(nil)
}

// verifyChain checks the server's certificate chain against the roots and
// the server name, unless the configuration skips it.
func (c *Conn) verifyChain(certs []*x509.Certificate) error {
	if c.config.InsecureSkipVerify {
		return nil
	}
	roots := c.config.RootCAs
	if roots == nil {
		var err error
		if roots, err = defaultRoots(); err != nil {
			return err
		}
	}
	inter := x509.NewCertPool()
	for _, cert := range certs[1:] {
		inter.AddCert(cert)
	}
	now := c.config.Time
	if now == nil {
		now = timeNow
	}
	_, err := certs[0].Verify(x509.VerifyOptions{
		DNSName:       c.config.ServerName,
		Roots:         roots,
		Intermediates: inter,
		CurrentTime:   now(),
	})
	return err
}
