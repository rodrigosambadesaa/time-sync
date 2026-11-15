package main

import (
    "context"
    "errors"
    "fmt"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
    "time"
)

var sshTargetPattern = regexp.MustCompile(`^root@[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

// Only root accounts on DNS names or IPv4 addresses are supported. These
// restrictions prevent SSH option/shell injection and ambiguous destinations.
func validateSSHTarget(target string) error {
    if !sshTargetPattern.MatchString(target) {
        return errors.New("-target-ssh must be root@hostname or root@IPv4, without whitespace or options")
    }
    return nil
}

type sshCommand func(command string) (string, error)

// sshRunner requires a pre-trusted SSH host key and key-based authentication.
// No password prompts, host-key acceptance prompts, or implicit trust-on-first-use.
func sshRunner(target string) sshCommand {
    return func(command string) (string, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
        defer cancel()
        output, err := exec.CommandContext(ctx, "ssh",
            "-o", "BatchMode=yes",
            "-o", "StrictHostKeyChecking=yes",
            "-o", "ConnectTimeout=6",
            "-o", "ConnectionAttempts=1",
            "-T", "--", target, command).CombinedOutput()
        if err != nil {
            if ctx.Err() != nil { return "", fmt.Errorf("SSH command timed out: %w", ctx.Err()) }
            return "", fmt.Errorf("SSH command failed: %w: %s", err, strings.TrimSpace(string(output)))
        }
        return string(output), nil
    }
}

// setRemoteClock is for jailbroken iOS devices with a working privileged
// 'date' binary and an intentionally configured root SSH service. It takes
// the authenticated offset from the controller; it does not contact NTP on
// the phone. It verifies the PHONE's clock step independently of host -max-step.
// BSD 'date' accepts MMDDhhmmYYYY.ss in UTC; remote updates have second
// precision and SSH latency, so this is not a high-precision PTP/NTP service.
func setRemoteClock(offset, maxStep time.Duration, run sshCommand) error {
    if maxStep <= 0 { return errors.New("maximum clock step must be positive") }
    start := time.Now()
    reply, err := run("date -u +%s")
    if err != nil { return fmt.Errorf("read remote clock: %w", err) }
    elapsed := time.Since(start)
    if elapsed > 10*time.Second { return errors.New("remote clock read took too long; refusing stale correction") }
    raw := strings.TrimSpace(reply)
    if raw == "" || strings.ContainsAny(raw, " \t\r\n") { return errors.New("unexpected remote clock output") }
    unixSeconds, err := strconv.ParseInt(raw, 10, 64)
    if err != nil { return fmt.Errorf("invalid remote Unix timestamp: %w", err) }
    remoteNow := time.Unix(unixSeconds, 0).Add(elapsed / 2)
    desired := time.Now().UTC().Add(offset)
    correction := desired.Sub(remoteNow)
    if correction > maxStep || correction < -maxStep {
        return fmt.Errorf("remote clock correction %s exceeds -max-step %s; clock left unchanged", correction, maxStep)
    }
    _, err = run("date -u " + desired.Format("010215042006.05"))
    if err != nil { return fmt.Errorf("set remote clock: %w", err) }
    return nil
}
