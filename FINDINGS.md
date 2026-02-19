# Findings

## Code Quality

- **76 tests in node/, all pass** — but only 42% statement coverage
- **logging/ fully tested** (12 tests, 100% coverage)
- **UEPS has ZERO tests** — 262 lines of crypto wire protocol completely untested
- **`go vet` clean** — no static analysis warnings
- **Zero TODOs/FIXMEs** in codebase

## Security Posture (Strong)

- X25519 ECDH key exchange with Borg STMF
- Challenge-response authentication (HMAC-SHA256)
- TLS 1.2+ with hardened cipher suites
- Message deduplication (5-min TTL, prevents amplification)
- Per-peer rate limiting (100 burst, 50 msg/sec)
- Tarball extraction: Zip Slip defence, 100 MB per-file limit, symlink/hardlink rejection
- Peer auth modes: open or public-key allowlist

## Architecture Strengths

- Clean separation: identity / transport / peers / protocol / worker / controller
- KD-tree peer selection via Poindexter: [PingMS × 1.0, Hops × 0.7, GeoKM × 0.2, (100-Score) × 1.2]
- Debounced persistence (5s coalesce window for peer registry)
- Buffer pool for JSON encoding (reduces GC pressure)
- Decoupled MinerManager/ProfileManager interfaces

## Critical Test Gaps

| File | Lines | Tests | Coverage |
|------|-------|-------|----------|
| identity.go | 290 | 5 tests | Good |
| peer.go | 708 | 19 tests | Good |
| message.go | 237 | 8 tests | Good |
| worker.go | 402 | 10 tests | Good |
| bundle.go | 355 | 9 tests | Good |
| protocol.go | 88 | 5 tests | Good |
| **transport.go** | **934** | **0 tests** | **NONE** |
| **controller.go** | **327** | **0 tests** | **NONE** |
| **dispatcher.go** | **39** | **stub** | **N/A** |
| **ueps/packet.go** | **124** | **0 tests** | **NONE** |
| **ueps/reader.go** | **138** | **0 tests** | **NONE** |

## Known Issues

1. **dispatcher.go is a stub** — Contains commented-out UEPS routing code. Threat circuit breaker and intent routing not implemented.
2. **UEPS 0xFF payload length ambiguous** — Relies on external TCP framing, not self-delimiting. Comments note this but no solution implemented.
3. **Potential race in controller.go** — `transport.OnMessage(c.handleResponse)` called during init; if message arrives before pending map ready, could theoretically panic (unlikely in practice).
4. **No resource cleanup on some error paths** — transport.handleWSUpgrade doesn't clean up on handshake timeout; transport.Connect doesn't clean up temp connection on error.
5. **Threat score semantics undefined** — Referenced in dispatcher stub and UEPS header but no scoring/routing logic exists.
