package main

import (
	"fmt"
	"math/big"
	"time"

	"ps2go/lib/harness"
)

func n(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("bad number " + s)
	}
	return v
}

// testBigArith checks math/big against Python-computed values (32-bit
// Words on this target).
func testBigArith() error {
	a := n("123456789012345678901234567890123456789")
	b := n("987654321098765432109876543210987654321")
	checks := []struct {
		name string
		got  *big.Int
		want string
	}{
		{"add", new(big.Int).Add(a, b), "1111111110111111111011111111101111111110"},
		{"sub", new(big.Int).Sub(b, a), "864197532086419753208641975320864197532"},
		{"mul", new(big.Int).Mul(a, b), "121932631137021795226185032733866788594487120865336229233322374638011112635269"},
		{"quo", new(big.Int).Quo(b, a), "8"},
		{"rem", new(big.Int).Rem(b, a), "9000000000900000000090000000009"},
		{"exp", new(big.Int).Exp(n("3"), n("200"), nil), "265613988875874769338781322035779626829233452653394495974574961739092490901302182994384699044001"},
		{"modexp", new(big.Int).Exp(n("2"), n("1000"), n("1000000007")), "688423210"},
		{"gcd", new(big.Int).GCD(nil, nil, n("462"), n("1071")), "21"},
		{"modinv", new(big.Int).ModInverse(n("3"), n("1000000007")), "333333336"},
		{"sqrt", new(big.Int).Sqrt(n("152415787532388367504953515625666819450083828733760")), "12345678901234567890123456"},
		{"lsh", new(big.Int).Lsh(n("1"), 100), "1267650600228229401496703205376"},
		{"neg-quo", new(big.Int).Quo(n("-7"), n("2")), "-3"},
		{"neg-mod", new(big.Int).Mod(n("-7"), n("2")), "1"},
	}
	for _, c := range checks {
		if c.got.String() != c.want {
			return fmt.Errorf("%s: got %s want %s", c.name, c.got, c.want)
		}
	}
	if s := new(big.Int).Mul(a, b).Text(16); s != "10d936c6dd454c29c60200b8f07db6a2cc48ee37874bd5a6e7df6807d34223b85" {
		return fmt.Errorf("hex %s", s)
	}
	f := new(big.Float).SetPrec(200).Quo(big.NewFloat(1), big.NewFloat(3))
	harness.Logf("1/3 at 200 bits: %s", f.Text('g', 40))
	r := big.NewRat(3, 9)
	if r.String() != "1/3" {
		return fmt.Errorf("rat %s", r)
	}
	return nil
}

// testBigPerf times RSA-sized operations (about 0.5 ms, 3.4 s and 22 ms
// in PCSX2).
func testBigPerf() error {
	x := new(big.Int).Lsh(n("1"), 2047)
	x.Sub(x, n("12345"))
	y := new(big.Int).Lsh(n("1"), 2047)
	y.Sub(y, n("67891"))
	t := time.Now()
	for i := 0; i < 100; i++ {
		new(big.Int).Mul(x, y)
	}
	mul := time.Since(t) / 100
	m := new(big.Int).Lsh(n("1"), 2048)
	m.Sub(m, n("189")) // odd modulus
	e := new(big.Int).Lsh(n("1"), 2047)
	e.Add(e, n("1"))
	t = time.Now()
	new(big.Int).Exp(x, e, m)
	modexp := time.Since(t)
	t = time.Now()
	new(big.Int).Exp(x, n("65537"), m)
	rsaPub := time.Since(t)
	harness.Logf("2048-bit mul %d us, 2048-bit modexp (RSA private op size) %d ms, e=65537 modexp (RSA verify) %d ms", mul.Microseconds(), modexp.Milliseconds(), rsaPub.Milliseconds())
	return nil
}
