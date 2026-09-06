package tls13

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Record content types.
const (
	recordChangeCipherSpec = 20
	recordAlert            = 21
	recordHandshake        = 22
	recordApplicationData  = 23
)

const (
	maxPlaintext  = 16384
	maxCiphertext = maxPlaintext + 256
)

// Alert descriptions we send or name.
const (
	alertCloseNotify        = 0
	alertUnexpectedMessage  = 10
	alertBadRecordMAC       = 20
	alertRecordOverflow     = 22
	alertHandshakeFailure   = 40
	alertBadCertificate     = 42
	alertCertificateExpired = 45
	alertCertificateUnknown = 46
	alertIllegalParameter   = 47
	alertUnknownCA          = 48
	alertDecodeError        = 50
	alertDecryptError       = 51
	alertProtocolVersion    = 70
	alertInternalError      = 80
	alertMissingExtension   = 109
	alertUnsupportedExt     = 110
	alertUnrecognizedName   = 112
	alertCertRequired       = 116
	alertNoApplicationProto = 120
)

// AlertError is a fatal alert received from the peer.
type AlertError uint8

func (e AlertError) Error() string {
	return fmt.Sprintf("tls13: alert from peer: %d", uint8(e))
}

// readRecord reads one record into c.rawInput, decrypting it when read
// keys are set. It returns the content type and the plaintext.
func (c *Conn) readRecord() (byte, []byte, error) {
	if _, err := io.ReadFull(c.conn, c.hdr[:]); err != nil {
		return 0, nil, err
	}
	typ := c.hdr[0]
	n := int(binary.BigEndian.Uint16(c.hdr[3:]))
	if n > maxCiphertext {
		return 0, nil, c.fail(alertRecordOverflow, errors.New("tls13: record too large"))
	}
	if cap(c.rawInput) < n {
		c.rawInput = make([]byte, n)
	}
	buf := c.rawInput[:n]
	if _, err := io.ReadFull(c.conn, buf); err != nil {
		return 0, nil, err
	}
	if c.in == nil || typ == recordChangeCipherSpec {
		// Plaintext (before the handshake keys) or the compatibility
		// change_cipher_spec, which is always ignored.
		return typ, buf, nil
	}
	if typ != recordApplicationData {
		return 0, nil, c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: unexpected record type %d", typ))
	}
	plain, err := c.in.aead.Open(buf[:0], c.in.nonce(), buf, c.hdr[:])
	c.in.seq++
	if err != nil {
		return 0, nil, c.fail(alertBadRecordMAC, errors.New("tls13: bad record MAC"))
	}
	// Strip the padding and take the inner content type.
	i := len(plain) - 1
	for i >= 0 && plain[i] == 0 {
		i--
	}
	if i < 0 {
		return 0, nil, c.fail(alertUnexpectedMessage, errors.New("tls13: empty inner record"))
	}
	return plain[i], plain[:i], nil
}

// writeRecord sends one record of the type, encrypting it when write keys
// are set. data must be at most maxPlaintext bytes.
func (c *Conn) writeRecord(typ byte, data []byte) error {
	var out []byte
	if c.out == nil {
		out = make([]byte, 5+len(data))
		out[0] = typ
		out[1], out[2] = 3, 3
		binary.BigEndian.PutUint16(out[3:], uint16(len(data)))
		copy(out[5:], data)
	} else {
		inner := make([]byte, len(data)+1)
		copy(inner, data)
		inner[len(data)] = typ
		n := len(inner) + c.out.aead.Overhead()
		out = make([]byte, 5, 5+n)
		out[0] = recordApplicationData
		out[1], out[2] = 3, 3
		binary.BigEndian.PutUint16(out[3:], uint16(n))
		out = c.out.aead.Seal(out, c.out.nonce(), inner, out[:5])
		c.out.seq++
	}
	_, err := c.conn.Write(out)
	return err
}

// sendAlert sends a fatal alert (best effort) and records the error.
func (c *Conn) fail(alert uint8, err error) error {
	if c.err == nil {
		c.err = err
		c.writeRecord(recordAlert, []byte{2, alert})
	}
	return err
}

// readHandshake returns the next handshake message (type and body),
// reassembled across records.
func (c *Conn) readHandshake() (byte, []byte, error) {
	for {
		if len(c.hsBuf) >= 4 {
			n := int(c.hsBuf[1])<<16 | int(c.hsBuf[2])<<8 | int(c.hsBuf[3])
			if len(c.hsBuf) >= 4+n {
				typ := c.hsBuf[0]
				body := append([]byte(nil), c.hsBuf[4:4+n]...)
				c.hsBuf = c.hsBuf[4+n:]
				return typ, body, nil
			}
		}
		typ, data, err := c.readRecord()
		if err != nil {
			return 0, nil, err
		}
		switch typ {
		case recordHandshake:
			c.hsBuf = append(c.hsBuf, data...)
		case recordChangeCipherSpec:
			// ignored
		case recordAlert:
			if len(data) == 2 && data[0] == 2 {
				return 0, nil, AlertError(data[1])
			}
			return 0, nil, fmt.Errorf("tls13: alert %v", data)
		default:
			return 0, nil, c.fail(alertUnexpectedMessage, fmt.Errorf("tls13: record type %d during handshake", typ))
		}
	}
}
