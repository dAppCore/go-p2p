# Findings

## Code Quality

- **90 tests in node/, all pass** — 72.1% statement coverage (up from 63.5%)
- **logging/ fully tested** (12 tests, 100% coverage)
- **UEPS 88.5% coverage** — wire protocol tests added in Phase 1
- **`go vet` clean** — no static analysis warnings
- **`go test -race` clean** — no data races (GracefulClose race fixed, see below)
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
| transport.go | 934 | 11 tests | Good (Phase 2) |
| controller.go | 327 | 14 tests | Good (Phase 3) |
| **dispatcher.go** | **39** | **stub** | **N/A** |
| ueps/packet.go | 124 | 9 tests | Good (Phase 1) |
| ueps/reader.go | 138 | 9 tests | Good (Phase 1) |

## Known Issues

1. **dispatcher.go is a stub** — Contains commented-out UEPS routing code. Threat circuit breaker and intent routing not implemented.
2. **UEPS 0xFF payload length ambiguous** — Relies on external TCP framing, not self-delimiting. Comments note this but no solution implemented.
3. **~~Potential race in controller.go~~** — ~~`transport.OnMessage(c.handleResponse)` called during init~~ — Not a real issue. The pending map is initialised in `NewController` before `OnMessage` is called, and `handleResponse` uses a mutex. No panic possible.
4. **No resource cleanup on some error paths** — transport.handleWSUpgrade doesn't clean up on handshake timeout; transport.Connect doesn't clean up temp connection on error.
5. **Threat score semantics undefined** — Referenced in dispatcher stub and UEPS header but no scoring/routing logic exists.

## Bugs Fixed

1. **P2P-RACE-1: GracefulClose data race** (Phase 3) — `GracefulClose` called `pc.Conn.SetWriteDeadline()` outside of `writeMu`, racing with concurrent `Send()` calls that also modify the write deadline. Fixed by removing the bare `SetWriteDeadline` call and relying on `Send()` which already manages deadlines under the lock. Detected by `go test -race`.
