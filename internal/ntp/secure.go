package ntp

import (
    "context"
    "crypto/tls"
    "net"
    "fmt"
    "time"

    beevikntp "github.com/beevik/ntp"
    "github.com/beevik/nts"
)

// QueryNTS obtains a cryptographically authenticated time sample using RFC 8915.
// A TLS 1.3 handshake is not enough: the NTP reply must also authenticate via
// NTS AEAD and pass normal NTP sanity checks. There is NO insecure fallback.
func QueryNTS(ctx context.Context, host string) (Sample, error) {
    sample := Sample{Server: host}
    if err := ctx.Err(); err != nil { return sample, err }
    // NTS verifies the X.509 certificate, establishes TLS 1.3 NTS-KE
    // keys and verifies NTP AEAD and anti-replay token.
    session, err := nts.NewSessionWithOptions(host, &nts.SessionOptions{
        Timeout: 3 * time.Second,
        // Bound the entire NTS-KE exchange, not just the TCP connection setup.
        // Keep standard TLS hostname/certificate validation and the NTS ALPN.
        Dialer: func(network, addr string, tlsConfig *tls.Config) (*tls.Conn, error) {
            conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, network, addr, tlsConfig)
            if err != nil { return nil, err }
            if err = conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
                _ = conn.Close()
                return nil, err
            }
            return conn, nil
        },
    })
    if err != nil { return sample, fmt.Errorf("NTS key exchange (%s): %w", host, err) }
    if err := ctx.Err(); err != nil { return sample, err }
    response, err := session.QueryWithOptions(&beevikntp.QueryOptions{Timeout: 2 * time.Second, Version: 4})
    if err != nil { return sample, fmt.Errorf("authenticated NTP exchange (%s): %w", host, err) }
    if err := response.Validate(); err != nil { return sample, fmt.Errorf("invalid authenticated time sample (%s): %w", host, err) }
    if response.Stratum == 0 || response.Stratum > 15 { return sample, fmt.Errorf("invalid NTS server stratum %d", response.Stratum) }
    if response.RTT < 0 || response.RTT > 600*time.Millisecond {
        return sample, fmt.Errorf("unacceptable authenticated round-trip delay: %s", response.RTT)
    }
    if response.RootDispersion > 2*time.Second {
        return sample, fmt.Errorf("unreliable authenticated root dispersion: %s", response.RootDispersion)
    }
    if err := ctx.Err(); err != nil { return sample, err }
    sample.Offset, sample.RTT, sample.Stratum = response.ClockOffset, response.RTT, response.Stratum
    sample.Authenticated = true // Only after authentication and validation.
    return sample, nil
}
