# Findings

## Code Quality

- **node/ 87.5% statement coverage** — integration tests, bufpool tests, and benchmarks added (Phase 5)
- **ueps/ 93.1% coverage** — benchmarks added (Phase 5)
- **logging/ fully tested** (12 tests, 86.2% coverage)
- **`go vet` clean** — no static analysis warnings
- **`go test -race` clean** — no data races
- **Zero TODOs/FIXMEs** in codebase

## Security Posture (Strong)

- X25519 ECDH key exchange with Borg STMF
- Challenge-response authentication (HMAC-SHA256)
- TLS 1.2+ with hardened cipher suites
- Message deduplication (5-min TTL, prevents amplification)
- Per-peer rate limiting (100 burst, 50 msg/sec)
- Tarball extraction: Zip Slip defence, 100 MB per-file limit, symlink/hardlink rejection
- Peer auth modes: open or public-key allowlist
- UEPS threat circuit breaker: packets with ThreatScore > 50,000 dropped before intent routing

## Architecture Strengths

- Clean separation: identity / transport / peers / protocol / worker / controller
- KD-tree peer selection via Poindexter: [PingMS × 1.0, Hops × 0.7, GeoKM × 0.2, (100-Score) × 1.2]
- Debounced persistence (5s coalesce window for peer registry)
- Buffer pool for JSON encoding (reduces GC pressure)
- Decoupled MinerManager/ProfileManager interfaces
- UEPS dispatcher: functional IntentHandler type, RWMutex-protected handler map, sentinel errors

## Test Coverage Summary

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
| dispatcher.go | 120 | 10 tests (17 subtests) | 100% (Phase 4) |
| bufpool.go | 55 | 11 tests | Good (Phase 5) |
| integration | — | 10 tests | Good (Phase 5) |
| ueps/packet.go | 124 | 9 tests | Good (Phase 1) |
| ueps/reader.go | 138 | 9 tests | Good (Phase 1) |

## Known Issues

1. ~~**dispatcher.go is a stub**~~ — Fully implemented (Phase 4). Threat circuit breaker and intent routing operational.
2. **UEPS 0xFF payload length ambiguous** — Relies on external TCP framing, not self-delimiting. Comments note this but no solution implemented.
3. **~~Potential race in controller.go~~** — ~~`transport.OnMessage(c.handleResponse)` called during init~~ — Not a real issue. The pending map is initialised in `NewController` before `OnMessage` is called, and `handleResponse` uses a mutex. No panic possible.
4. **No resource cleanup on some error paths** — transport.handleWSUpgrade doesn't clean up on handshake timeout; transport.Connect doesn't clean up temp connection on error.
5. ~~**Threat score semantics undefined**~~ — ThreatScoreThreshold (50,000) defined in dispatcher. Packets above threshold dropped and logged. Intent routing implemented for 0x01/0x20/0x30/0xFF.

## Phase 4 Design Decisions

1. **IntentHandler as func type, not interface** — Matches the codebase's `MessageHandler` pattern in transport.go. Lighter weight than an interface for a single-method contract.
2. **Sentinel errors over silent drops** — The stub comments suggested silent drops, but returning typed errors (`ErrThreatScoreExceeded`, `ErrUnknownIntent`, `ErrNilPacket`) gives callers the option to inspect outcomes. The dispatcher still logs at WARN level regardless.
3. **Threat check before intent routing** — A high-threat packet with an unknown intent returns `ErrThreatScoreExceeded`, not `ErrUnknownIntent`. The circuit breaker is the first line of defence; no packet metadata is inspected beyond ThreatScore before the drop.
4. **Threshold at 50,000 (not configurable)** — Kept as a constant to match the original stub. Can be made configurable via functional options if needed later.
5. **RWMutex for handler map** — Read-heavy workload (dispatches far outnumber registrations), so RWMutex is appropriate. Registration takes a write lock, dispatch takes a read lock.

## Phase 5 Benchmark Results (AMD Ryzen 9 9950X)

| Operation | Time/op | Allocs/op |
|-----------|---------|-----------|
| Identity key generation (STMF) | 23 us | 6 |
| Full identity generation + disk | 74 us | 51 |
| Shared secret derivation (ECDH) | 46 us | 9 |
| KD-tree nearest (10 peers) | 247 ns | 2 |
| KD-tree nearest (100 peers) | 497 ns | 2 |
| KD-tree nearest (1000 peers) | 2.9 us | 2 |
| NewMessage (Ping) | 271 ns | 5 |
| NewMessage (Stats, 2 miners) | 701 ns | 5 |
| MarshalJSON (pooled buffer) | 375 ns | 2 |
| json.Marshal (stdlib) | 367 ns | 2 |
| SMSG Encrypt | 2.6 us | 34 |
| SMSG Decrypt | 3.9 us | 47 |
| SMSG Round-trip | 6.4 us | 81 |
| Challenge generate | 105 ns | 1 |
| Challenge sign (HMAC-SHA256) | 276 ns | 6 |
| Challenge verify | 295 ns | 6 |
| UEPS marshal+sign (64B payload) | 509 ns | 27 |
| UEPS marshal+sign (1KB payload) | 948 ns | 27 |
| UEPS marshal+sign (64KB payload) | 27.7 us | 27 |
| UEPS read+verify (64B payload) | 843 ns | 18 |
| UEPS read+verify (1KB payload) | 1.4 us | 20 |
| UEPS read+verify (64KB payload) | 44 us | 33 |
| UEPS round-trip (256B payload) | 1.5 us | 45 |

**Observations:**
- KD-tree peer scoring scales well: O(log n) with only 2 allocs regardless of tree size.
- MarshalJSON's buffer pool shows no measurable advantage at small message sizes — stdlib has been heavily optimised. The pool's value emerges under sustained load where GC pressure is reduced.
- SMSG is the dominant cost per-message (~6.4 us round-trip), making it the primary bottleneck for message throughput. At ~150K encrypted messages/sec/core, this is adequate for P2P mesh traffic.
- UEPS wire format is lightweight: signing a 64B packet costs ~500 ns, mostly HMAC computation.

## Bugs Fixed

1. **P2P-RACE-1: GracefulClose data race** (Phase 3) — `GracefulClose` called `pc.Conn.SetWriteDeadline()` outside of `writeMu`, racing with concurrent `Send()` calls that also modify the write deadline. Fixed by removing the bare `SetWriteDeadline` call and relying on `Send()` which already manages deadlines under the lock. Detected by `go test -race`.
