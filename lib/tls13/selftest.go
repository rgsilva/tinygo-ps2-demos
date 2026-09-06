package tls13

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// scriptedConn hands out scripted bytes and records what is written.
type scriptedConn struct {
	in  bytes.Buffer
	out bytes.Buffer
}

func (s *scriptedConn) Read(b []byte) (int, error)         { return s.in.Read(b) }
func (s *scriptedConn) Write(b []byte) (int, error)        { return s.out.Write(b) }
func (s *scriptedConn) Close() error                       { return nil }
func (s *scriptedConn) LocalAddr() net.Addr                { return nil }
func (s *scriptedConn) RemoteAddr() net.Addr               { return nil }
func (s *scriptedConn) SetDeadline(t time.Time) error      { return nil }
func (s *scriptedConn) SetReadDeadline(t time.Time) error  { return nil }
func (s *scriptedConn) SetWriteDeadline(t time.Time) error { return nil }

type countingRand struct{}

func (countingRand) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = byte(i)
	}
	return len(b), nil
}

// SelfTest checks the primitives and the handshake against known answers
// (FIPS 180, NIST GCM, RFC 8439, RFC 7748, RFC 5869 and the RFC 8448
// handshake replayed bit for bit). It names the first thing that differs:
// on a new target that is the fastest way to tell a compiler bug from a
// protocol bug.
func SelfTest() error {
	if got := sha256.Sum256([]byte("abc")); !bytes.Equal(got[:], mustHex("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")) {
		return fmt.Errorf("sha256: %x", got)
	}
	if got := sha512.Sum384([]byte("abc")); !bytes.Equal(got[:], mustHex("cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7")) {
		return fmt.Errorf("sha384: %x", got)
	}
	if got := sha512.Sum384([]byte("abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrstu")); !bytes.Equal(got[:], mustHex("09330c33f71147e83d192fc782cd1b4753111b173b3b05d22fa08086e3b0f712fcc7c71a557e2db966c3e9fa91746039")) {
		return fmt.Errorf("sha384 (two blocks): %x", got)
	}
	// RFC 4231 test case 2, HMAC-SHA-384, through the suite's extract.
	s384 := suiteByID(TLS_AES_256_GCM_SHA384)
	if got := s384.extract([]byte("Jefe"), []byte("what do ya want for nothing?")); !bytes.Equal(got, mustHex("af45d2e376484031617f78d2b58a6b1b9c7ef464f5a01b47e42ec3736322445e8e2240ca5e69e2c78b3239ecfab21649")) {
		return fmt.Errorf("hmac-sha384: %x", got)
	}
	// NIST GCM test case 16: AES-256 with data and AAD.
	aead, err := aesGCM(mustHex("feffe9928665731c6d6a8f9467308308feffe9928665731c6d6a8f9467308308"))
	if err != nil {
		return err
	}
	if got := aead.Seal(nil, mustHex("cafebabefacedbaddecaf888"), mustHex("d9313225f88406e5a55909c5aff5269a86a7a9531534f7da2e4c303d8a318a721c3c0c95956809532fcf0e2449a6b525b16aedf5aa0de657ba637b39"), mustHex("feedfacedeadbeeffeedfacedeadbeefabaddad2")); !bytes.Equal(got, mustHex("522dc1f099567d07f47f37a32a84427d643a8cdcbfe5c0c97598a2bd2555d1aa8cb08e48590dbb3da7b08b1056828838c5f61e6393ba7a0abcc9f66276fc6ece0f4e1768cddf8853bb2d551b")) {
		return fmt.Errorf("aes-256-gcm (case 16): %x", got)
	}
	// NIST GCM test case 2: zero key, zero IV, one zero block.
	aead, err = aesGCM(make([]byte, 16))
	if err != nil {
		return err
	}
	if got := aead.Seal(nil, make([]byte, 12), make([]byte, 16), nil); !bytes.Equal(got, mustHex("0388dace60b6a392f328c2b971b2fe78ab6e47d42cec13bdf53a67b21257bddf")) {
		return fmt.Errorf("aes-128-gcm: %x", got)
	}
	// NIST GCM test case 14: zero 256-bit key, zero IV, one zero block.
	aead, err = aesGCM(make([]byte, 32))
	if err != nil {
		return err
	}
	if got := aead.Seal(nil, make([]byte, 12), make([]byte, 16), nil); !bytes.Equal(got, mustHex("cea7403d4d606b6e074ec5d3baf39d18d0d1c8a799996bf0265b98b5d48ab919")) {
		return fmt.Errorf("aes-256-gcm: %x", got)
	}
	// RFC 8439 section 2.8.2.
	c20, err := chacha20poly1305.New(mustHex("808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f"))
	if err != nil {
		return err
	}
	pt := []byte("Ladies and Gentlemen of the class of '99: If I could offer you only one tip for the future, sunscreen would be it.")
	want := mustHex("d31a8d34648e60db7b86afbc53ef7ec2a4aded51296e08fea9e2b5a736ee62d63dbea45e8ca9671282fafb69da92728b1a71de0a9e060b2905d6a5b67ecd3b3692ddbd7f2d778b8c9803aee328091b58fab324e4fad675945585808b4831d7bc3ff4def08e4b7a9de576d26586cec64b61161ae10b594f09e26a7e902ecbd0600691")
	if got := c20.Seal(nil, mustHex("070000004041424344454647"), pt, mustHex("50515253c0c1c2c3c4c5c6c7")); !bytes.Equal(got, want) {
		return fmt.Errorf("chacha20-poly1305: %x", got)
	}
	// RFC 5869 test case 1.
	s256 := suiteByID(TLS_AES_128_GCM_SHA256)
	prk := s256.extract(mustHex("000102030405060708090a0b0c"), mustHex("0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b"))
	if got := s256.expand(prk, mustHex("f0f1f2f3f4f5f6f7f8f9"), 42); !bytes.Equal(got, mustHex("3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865")) {
		return fmt.Errorf("hkdf: %x", got)
	}
	// RFC 7748 section 6.1.
	shares, err := (&Conn{testKey: mustHex("77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a")}).makeKeyShares([]uint16{groupX25519})
	if err != nil {
		return err
	}
	peer, err := shares[0].key.Curve().NewPublicKey(mustHex("de9edb7d7b7dc1b4d35b61c2ece435373f8343c85b78674dadfc7e146f882b4f"))
	if err != nil {
		return err
	}
	if got, err := shares[0].key.ECDH(peer); err != nil || !bytes.Equal(got, mustHex("4a5d9d5ba4ce2de1728e3bf480350f25e07e21c947d19e3376f09b3c1e161742")) {
		return fmt.Errorf("x25519: %x %v", got, err)
	}
	// The RFC 8448 handshake, replayed.
	sc := &scriptedConn{}
	for _, k := range []string{"serverHelloRecord", "serverFlightRecord", "serverTicketRecord", "serverAppRecord", "serverAlertRecord"} {
		sc.in.Write(mustHex(rfc8448[k]))
	}
	c := Client(sc, &Config{ServerName: "server", InsecureSkipVerify: true, Rand: countingRand{}})
	c.testClientHello = mustHex(rfc8448["clientHello"])
	c.testKey = mustHex(rfc8448["clientPriv"])
	c.clientRandom = c.testClientHello[6:38]
	c.sessionID = []byte{}
	if err := c.Handshake(); err != nil {
		return fmt.Errorf("rfc 8448 handshake: %w", err)
	}
	want = append(append([]byte{recordHandshake, 3, 3, 0, byte(len(c.testClientHello))}, c.testClientHello...), mustHex(rfc8448["clientFinishedRecord"])...)
	if !bytes.Equal(sc.out.Bytes(), want) {
		return fmt.Errorf("rfc 8448 client records differ")
	}
	sc.out.Reset()
	if _, err := c.Write(mustHex(rfc8448["clientAppPayload"])); err != nil {
		return err
	}
	if !bytes.Equal(sc.out.Bytes(), mustHex(rfc8448["clientAppRecord"])) {
		return fmt.Errorf("rfc 8448 application record differs")
	}
	buf := make([]byte, 100)
	n, err := c.Read(buf)
	if err != nil || !bytes.Equal(buf[:n], mustHex(rfc8448["clientAppPayload"])) {
		return fmt.Errorf("rfc 8448 server application data: %x %v", buf[:n], err)
	}
	return nil
}
