package tls13

import (
	"crypto/x509"
	_ "embed"
	"errors"
)

// The root bundle: Mozilla's, as distributed by curl (https://curl.se/ca/).
//
//go:embed roots.pem
var rootsPEM []byte

var (
	rootPool    *x509.CertPool
	rootPoolErr error
)

// defaultRoots parses the bundle once.
func defaultRoots() (*x509.CertPool, error) {
	if rootPool == nil && rootPoolErr == nil {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(rootsPEM) {
			rootPoolErr = errors.New("tls13: no roots in the bundle")
		} else {
			rootPool = pool
		}
	}
	return rootPool, rootPoolErr
}
