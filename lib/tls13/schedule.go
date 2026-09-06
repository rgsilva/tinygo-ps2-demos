package tls13

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	"golang.org/x/crypto/chacha20poly1305"
)

// Cipher suites (RFC 8446 appendix B.4), in the order they are offered.
const (
	TLS_AES_128_GCM_SHA256       uint16 = 0x1301
	TLS_AES_256_GCM_SHA384       uint16 = 0x1302
	TLS_CHACHA20_POLY1305_SHA256 uint16 = 0x1303
)

type cipherSuite struct {
	id     uint16
	hash   crypto.Hash
	keyLen int
	aead   func(key []byte) (cipher.AEAD, error)
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

var cipherSuites = []*cipherSuite{
	{TLS_AES_128_GCM_SHA256, crypto.SHA256, 16, aesGCM},
	{TLS_CHACHA20_POLY1305_SHA256, crypto.SHA256, 32, chacha20poly1305.New},
	{TLS_AES_256_GCM_SHA384, crypto.SHA384, 32, aesGCM},
}

func suiteByID(id uint16) *cipherSuite {
	for _, s := range cipherSuites {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (s *cipherSuite) newHash() hash.Hash {
	if s.hash == crypto.SHA384 {
		return sha512.New384()
	}
	return sha256.New()
}

// HKDF (RFC 5869) on the suite's hash.

func (s *cipherSuite) extract(salt, ikm []byte) []byte {
	if salt == nil {
		salt = make([]byte, s.hash.Size())
	}
	if ikm == nil {
		ikm = make([]byte, s.hash.Size())
	}
	m := hmac.New(s.newHash, salt)
	m.Write(ikm)
	return m.Sum(nil)
}

func (s *cipherSuite) expand(prk, info []byte, n int) []byte {
	out := make([]byte, 0, n)
	var prev []byte
	for i := byte(1); len(out) < n; i++ {
		m := hmac.New(s.newHash, prk)
		m.Write(prev)
		m.Write(info)
		m.Write([]byte{i})
		prev = m.Sum(nil)
		out = append(out, prev...)
	}
	return out[:n]
}

// expandLabel is HKDF-Expand-Label (RFC 8446 section 7.1).
func (s *cipherSuite) expandLabel(secret []byte, label string, context []byte, n int) []byte {
	info := make([]byte, 0, 2+1+6+len(label)+1+len(context))
	info = append(info, byte(n>>8), byte(n))
	info = append(info, byte(6+len(label)))
	info = append(info, "tls13 "...)
	info = append(info, label...)
	info = append(info, byte(len(context)))
	info = append(info, context...)
	return s.expand(secret, info, n)
}

// deriveSecret is Derive-Secret: expandLabel with a transcript hash as the
// context.
func (s *cipherSuite) deriveSecret(secret []byte, label string, transcript []byte) []byte {
	return s.expandLabel(secret, label, transcript, s.hash.Size())
}

// finishedMAC is the verify_data of a Finished message for the base key.
func (s *cipherSuite) finishedMAC(baseKey, transcript []byte) []byte {
	key := s.expandLabel(baseKey, "finished", nil, s.hash.Size())
	m := hmac.New(s.newHash, key)
	m.Write(transcript)
	return m.Sum(nil)
}

// trafficKeys are one direction's record protection state.
type trafficKeys struct {
	secret []byte
	aead   cipher.AEAD
	iv     []byte
	seq    uint64
}

func (s *cipherSuite) trafficKeys(secret []byte) (*trafficKeys, error) {
	key := s.expandLabel(secret, "key", nil, s.keyLen)
	iv := s.expandLabel(secret, "iv", nil, 12)
	aead, err := s.aead(key)
	if err != nil {
		return nil, err
	}
	return &trafficKeys{secret: secret, aead: aead, iv: iv}, nil
}

// nonce is the per-record nonce: the IV XOR the sequence number.
func (k *trafficKeys) nonce() []byte {
	n := make([]byte, 12)
	copy(n, k.iv)
	seq := k.seq
	for i := 11; i >= 4; i-- {
		n[i] ^= byte(seq)
		seq >>= 8
	}
	return n
}

// next is the key update: the next generation of this direction's keys.
func (s *cipherSuite) nextKeys(k *trafficKeys) (*trafficKeys, error) {
	return s.trafficKeys(s.expandLabel(k.secret, "traffic upd", nil, s.hash.Size()))
}
