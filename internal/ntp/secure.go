package ntp

import (
    "context"
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
    session, err := nts.NewSessionWithOptions(host, &nts.SessionOptions{Timeout: 3 * time.Second})
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
