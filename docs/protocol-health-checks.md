# Protocol health-check support

SubShare health results describe the check that was actually performed. They
must not be interpreted as stronger protocol validation than the table below.

| Protocol | Current check | Successful result means |
| --- | --- | --- |
| VLESS | DNS/public-address policy, then TCP connect | The endpoint accepts a TCP connection. VLESS authentication and transport negotiation are not performed. |
| VMess | DNS/public-address policy, then TCP connect | The endpoint accepts a TCP connection. VMess authentication and transport negotiation are not performed. |
| Trojan | DNS/public-address policy, then TCP connect | The endpoint accepts a TCP connection. Trojan authentication and TLS/application negotiation are not performed. |
| Shadowsocks | DNS/public-address policy, then TCP connect | Authentication is not implemented, so even a reachable TCP port is recorded as `unsupported_check`. |
| Hysteria2 | DNS/public-address policy, then an official Hysteria2 QUIC/HTTP3 authentication handshake | The supplied profile authenticated successfully. TLS options, certificate pinning, port hopping, Salamander, and Gecko are applied from the normalized profile. |
| TUIC | No protocol-aware client is embedded | The result is `unsupported_check`; DNS or TCP reachability is not reported as protocol health. |
| Hysteria 1.x | No legacy client is embedded | The result is `unsupported_check`; it is never treated as Hysteria2. |

All probes have a four-second boundary and accept caller cancellation. Network
connections use only IP addresses approved by the external-destination resolver.
Stored diagnostics are stable reason codes and never include raw profiles or
credentials.

`unsupported_check` is an informational persisted state. Bulk checks count it
as `skipped_unsupported`; it does not increment health failures and does not by
itself make the job succeed with warnings or fail.
