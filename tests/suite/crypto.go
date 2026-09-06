package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"ps2go/lib/harness"
)

// The crypto a TLS 1.3 client needs, on the PS2 (crypto/rand has no
// Reader here, so keys come from a fixed source).
type fixed struct{ n byte }

func (f *fixed) Read(b []byte) (int, error) {
	for i := range b {
		f.n = f.n*13 + 7
		b[i] = f.n
	}
	return len(b), nil
}

func testCryptoSymmetric() error {
	key := sha256.Sum256([]byte("key"))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, 12)
	msg := make([]byte, 16*1024)
	t := time.Now()
	ct := gcm.Seal(nil, nonce, msg, nil)
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil || len(pt) != len(msg) {
		return fmt.Errorf("gcm: %v", err)
	}
	took := time.Since(t)
	mac := hmac.New(sha256.New, key[:])
	mac.Write(msg)
	harness.Logf("AES-128-GCM seal+open of 16 KB: %d ms; hmac %x", took.Milliseconds(), mac.Sum(nil)[:4])
	return nil
}

func testCryptoECDH() error {
	t := time.Now()
	a, err := ecdh.X25519().GenerateKey(&fixed{1})
	if err != nil {
		return err
	}
	b, err := ecdh.X25519().GenerateKey(&fixed{2})
	if err != nil {
		return err
	}
	s1, err := a.ECDH(b.PublicKey())
	if err != nil {
		return err
	}
	s2, _ := b.ECDH(a.PublicKey())
	if string(s1) != string(s2) {
		return fmt.Errorf("shared secrets differ")
	}
	harness.Logf("X25519 keygen x2 + ECDH x2: %d ms", time.Since(t).Milliseconds())
	t = time.Now()
	p, err := ecdh.P256().GenerateKey(&fixed{3})
	if err != nil {
		return err
	}
	q, _ := ecdh.P256().GenerateKey(&fixed{4})
	if _, err := p.ECDH(q.PublicKey()); err != nil {
		return err
	}
	harness.Logf("P-256 keygen x2 + ECDH: %d ms", time.Since(t).Milliseconds())
	return nil
}

func testCryptoSignatures() error {
	t := time.Now()
	k, err := ecdsa.GenerateKey(elliptic.P256(), &fixed{5})
	if err != nil {
		return err
	}
	h := sha256.Sum256([]byte("msg"))
	sig, err := ecdsa.SignASN1(&fixed{6}, k, h[:])
	if err != nil {
		return err
	}
	t2 := time.Now()
	ok := ecdsa.VerifyASN1(&k.PublicKey, h[:], sig)
	harness.Logf("ECDSA P-256 keygen+sign %d ms, verify %d ms, ok %v", t2.Sub(t).Milliseconds(), time.Since(t2).Milliseconds(), ok)
	// RSA verify with a fixed 2048-bit modulus (keygen would take minutes).
	n, _ := new(big.Int).SetString("C4F8E9E15DCADF2B96C763D981006A644FFB4415030A16ED1283883340F2AA0E2BE2BE8FA60150B9046965837C3E7D151B7DE237EBB957C20663898250703B3F", 16)
	pub := &rsa.PublicKey{N: n, E: 65537}
	t = time.Now()
	rsa.VerifyPKCS1v15(pub, 0, h[:], make([]byte, 64)) // expected to fail: timing only
	harness.Logf("RSA-512 verify attempt %d ms", time.Since(t).Milliseconds())
	return nil
}

func testCryptoX509() error {
	block, _ := pem.Decode([]byte(testCert))
	if block == nil {
		return fmt.Errorf("no pem")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	harness.Logf("cert: %s, alg %v, until %s", c.Subject, c.SignatureAlgorithm, c.NotAfter.Format("2006-01-02"))
	pool := x509.NewCertPool()
	pool.AddCert(c)
	if _, err = c.Verify(x509.VerifyOptions{Roots: pool, CurrentTime: c.NotBefore.Add(time.Hour)}); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	return nil
}
