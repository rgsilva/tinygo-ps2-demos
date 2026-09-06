package tls13

import (
	"bytes"
	"encoding/hex"
	"io"
	"net"
	"testing"
	"time"
)

func h(t *testing.T, name string) []byte {
	b, err := hex.DecodeString(rfc8448[name])
	if err != nil {
		t.Fatal(name, err)
	}
	return b
}

// scriptConn is a net.Conn that hands the client scripted server bytes and
// records what the client writes.
type scriptConn struct {
	in  bytes.Buffer
	out bytes.Buffer
}

func (s *scriptConn) Read(b []byte) (int, error)         { return s.in.Read(b) }
func (s *scriptConn) Write(b []byte) (int, error)        { return s.out.Write(b) }
func (s *scriptConn) Close() error                       { return nil }
func (s *scriptConn) LocalAddr() net.Addr                { return nil }
func (s *scriptConn) RemoteAddr() net.Addr               { return nil }
func (s *scriptConn) SetDeadline(t time.Time) error      { return nil }
func (s *scriptConn) SetReadDeadline(t time.Time) error  { return nil }
func (s *scriptConn) SetWriteDeadline(t time.Time) error { return nil }

// fixedRand is a deterministic randomness source for the P-256 share.
type fixedRand struct{}

func (f *fixedRand) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = byte(i)
	}
	return len(b), nil
}

// TestKeySchedule derives the RFC 8448 secrets from its handshake messages.
func TestKeySchedule(t *testing.T) {
	suite := suiteByID(TLS_AES_128_GCM_SHA256)
	early := suite.extract(nil, nil)
	if !bytes.Equal(early, h(t, "earlySecret")) {
		t.Fatalf("early secret %x", early)
	}
	sh := h(t, "serverHelloRecord")[5:]
	tr := &transcript{}
	tr.start(suite)
	tr.add(h(t, "clientHello"))
	tr.add(sh)
	// The shared secret from the client's private key and the server's key
	// share in the ServerHello.
	shares, err := (&Conn{rand: &fixedRand{}, testKey: h(t, "clientPriv")}).makeKeyShares([]uint16{groupX25519})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseServerHello(sh[4:])
	if err != nil {
		t.Fatal(err)
	}
	peer, err := shares[0].key.Curve().NewPublicKey(parsed.peerKey)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := shares[0].key.ECDH(peer)
	if err != nil {
		t.Fatal(err)
	}
	hs := suite.extract(suite.deriveSecret(early, "derived", suite.hashEmpty()), shared)
	if !bytes.Equal(hs, h(t, "handshakeSecret")) {
		t.Fatalf("handshake secret %x", hs)
	}
	th := tr.sum()
	if got := suite.deriveSecret(hs, "c hs traffic", th); !bytes.Equal(got, h(t, "clientHS")) {
		t.Fatalf("client hs traffic %x", got)
	}
	serverHS := suite.deriveSecret(hs, "s hs traffic", th)
	if !bytes.Equal(serverHS, h(t, "serverHS")) {
		t.Fatalf("server hs traffic %x", serverHS)
	}
	keys, err := suite.trafficKeys(serverHS)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(keys.iv, h(t, "serverHSIV")) {
		t.Fatalf("server hs iv %x", keys.iv)
	}
	master := suite.extract(suite.deriveSecret(hs, "derived", suite.hashEmpty()), nil)
	if !bytes.Equal(master, h(t, "masterSecret")) {
		t.Fatalf("master %x", master)
	}
}

// TestHandshake runs the whole client handshake against the RFC's recorded
// server records and checks the client's bytes against the recording.
func TestHandshake(t *testing.T) {
	sc := &scriptConn{}
	sc.in.Write(h(t, "serverHelloRecord"))
	sc.in.Write(h(t, "serverFlightRecord"))
	sc.in.Write(h(t, "serverTicketRecord"))
	sc.in.Write(h(t, "serverAppRecord"))
	sc.in.Write(h(t, "serverAlertRecord"))
	c := Client(sc, &Config{ServerName: "server", InsecureSkipVerify: true, Rand: &fixedRand{}})
	c.testClientHello = h(t, "clientHello")
	c.testKey = h(t, "clientPriv")
	c.clientRandom = h(t, "clientHello")[6:38]
	c.sessionID = []byte{}
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	st := c.ConnectionState()
	if st.CipherSuite != TLS_AES_128_GCM_SHA256 || len(st.PeerCertificates) != 1 {
		t.Fatalf("state %+v", st)
	}
	want := append(append([]byte{recordHandshake, 3, 3, 0, byte(len(h(t, "clientHello")))}, h(t, "clientHello")...), h(t, "clientFinishedRecord")...)
	if !bytes.Equal(sc.out.Bytes(), want) {
		t.Fatalf("client sent\n%x\nwant\n%x", sc.out.Bytes(), want)
	}
	sc.out.Reset()
	if _, err := c.Write(h(t, "clientAppPayload")); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sc.out.Bytes(), h(t, "clientAppRecord")) {
		t.Fatalf("app record\n%x\nwant\n%x", sc.out.Bytes(), h(t, "clientAppRecord"))
	}
	got, err := io.ReadAll(c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, h(t, "clientAppPayload")) {
		t.Fatalf("server app data %x", got)
	}
}

func TestSelfTest(t *testing.T) {
	if err := SelfTest(); err != nil {
		t.Fatal(err)
	}
}
