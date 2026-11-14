package ntp

import (
    "context"
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/tls"
    "crypto/x509"
    "math/big"
    "testing"
    "time"
)

// A local NTS-KE service with an untrusted certificate must never become
// an authenticated source; no OS clock update is made.
func TestNTSRejectsUntrustedTLSCertificate(t *testing.T) {
    key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil { t.Fatal(err) }
    template := &x509.Certificate{
        SerialNumber: big.NewInt(42),
        NotBefore: time.Now().Add(-time.Hour),
        NotAfter: time.Now().Add(time.Hour),
        DNSNames: []string{"localhost"},
    }
    der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
    if err != nil { t.Fatal(err) }
    cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
    ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion: tls.VersionTLS13,
        NextProtos: []string{"ntske/1"},
    })
    if err != nil { t.Fatal(err) }
    defer ln.Close()
    done := make(chan struct{})
    go func() {
        defer close(done)
        c, err := ln.Accept()
        if err == nil {
            defer c.Close()
            _ = c.(*tls.Conn).Handshake()
        }
    }()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    sample, err := QueryNTS(ctx, ln.Addr().String())
    if err == nil || sample.Authenticated { t.Fatalf("untrusted NTS accepted: sample=%+v err=%v", sample, err) }
    _ = ln.Close()
    select { case <-done: case <-time.After(5*time.Second): t.Fatal("TLS fixture hung") }
}
