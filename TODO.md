# TODO.md — go-p2p Task Queue

Dispatched from core/go orchestration. Pick up tasks in phase order.

---

## Phase 1: UEPS Wire Protocol Tests — COMPLETE (88.5% coverage)

All crypto wire protocol tests implemented. Commit `2bc53ba`.

- [x] **PacketBuilder round-trip** — Basic, binary, threat score, large payload variants
- [x] **HMAC verification** — Payload tampering + header tampering both caught
- [x] **Wrong shared secret** — HMAC mismatch detected
- [x] **Empty payload** — Nil and empty slice both produce valid packets
- [x] **Max ThreatScore boundary** — uint16 max round-trips correctly
- [x] **Missing HMAC tag** — Error returned
- [x] **TLV value too large** — writeTLV error for >255 bytes
- [x] **Truncated packet** — EOF mid-TLV detected at multiple cut points
- [x] **Unknown TLV tag** — Reader skips unknown tags, included in signature

## Phase 2: Transport Tests — COMPLETE (node/ 42% → 63.5%)

All transport layer tests implemented with real WebSocket connections. Commit `3ee5553`.

- [x] **Test pair setup helper** — Reusable helper for identities + registries + transports
- [x] **Full handshake** — Challenge-response completes, shared secret derived
- [x] **Handshake rejection: wrong protocol version** — Rejection before disconnect
- [x] **Handshake rejection: allowlist** — "not authorized" rejection
- [x] **Encrypted message round-trip** — SMSG encrypt/decrypt verified
- [x] **Message deduplication** — Duplicate ID dropped silently
- [x] **Rate limiting** — Burst >100 messages, drops after token bucket empties
- [x] **MaxConns enforcement** — 503 rejection on overflow
- [x] **Keepalive timeout** — Connection cleaned up after PingInterval+PongTimeout
- [x] **Graceful close** — MsgDisconnect sent before close
- [x] **Concurrent sends** — No races (writeMu protects)

## Phase 3: Controller Tests — COMPLETE (node/ 63.5% → 72.1%)

All controller tests implemented with real WebSocket transport pairs. 14 tests total. Commit `33eda7b`.
Also fixed pre-existing data race in GracefulClose (P2P-RACE-1).

- [x] **Request-response correlation** — Send request, worker replies with ReplyTo set, controller matches correctly.
- [x] **Request timeout** — No response within deadline, returns timeout error.
- [x] **Auto-connect** — Peer not connected, controller auto-connects via transport before sending.
- [x] **GetAllStats** — Multiple connected peers, verify parallel stat collection completes.
- [x] **PingPeer RTT** — Send ping, receive pong, RTT calculated and peer metrics updated.
- [x] **Concurrent requests** — Multiple requests in flight to different peers, correct correlation.
- [x] **Dead peer cleanup** — Response channel cleaned up after timeout (no goroutine/memory leak).

## Phase 4: Dispatcher Implementation

Currently a commented-out stub in `node/dispatcher.go`. Implement once Phases 1-3 are solid.

- [ ] **Uncomment and implement DispatchUEPS** — Wire up to Transport for incoming UEPS packets.
- [ ] **Threat circuit breaker** — Drop packets with ThreatScore > 50000. Log as threat event.
- [ ] **Intent router** — Route by IntentID: 0x01 handshake, 0x20 compute, 0x30 rehab, 0xFF custom.
- [ ] **Dispatcher tests** — Unit tests for each intent route and threat rejection.

## Phase 5: Integration & Benchmarks

- [ ] **Full integration test** — Two nodes on localhost: identity creation, handshake, encrypted message exchange, UEPS packet routing, graceful shutdown.
- [ ] **Benchmarks** — Peer scoring (KD-tree), UEPS marshal/unmarshal, identity key generation, message serialisation, SMSG encrypt/decrypt.
- [ ] **bufpool.go tests** — Buffer reuse verification, concurrent access.

---

## Known Issues

1. **UEPS 0xFF payload has no length prefix** — Relies on external TCP framing (io.ReadAll reads to EOF). Not self-delimiting.
2. **Potential race in controller.go** — `transport.OnMessage(c.handleResponse)` called during init; message arriving before pending map is ready could theoretically panic.
3. **Resource cleanup gaps** — transport.handleWSUpgrade doesn't clean up on handshake timeout; transport.Connect doesn't clean up temp connection on error.
4. **Threat score semantics undefined** — Referenced in dispatcher stub and UEPS header but no scoring/routing logic exists.

## Wiki Inconsistencies Found (Charon, 19 Feb 2026)

Fixed in wiki update:
- ~~Node-Identity page says PublicKey is "hex-encoded"~~ — Code says base64 (identity.go:63)
- ~~Protocol-Messages page uses `Sender` field~~ — Code uses `From`/`To` (message.go:66-67)
- ~~Peer-Discovery page says Score is 0.0–1.0~~ — Code uses float64 range 0-100 (peer.go:31)

## Platform

- **OS**: Ubuntu (linux/amd64) — snider-linux
- **Co-located with**: go-rocm, go-rag

## Workflow

1. Charon dispatches tasks here after review
2. Pick up tasks in phase order
3. Mark `[x]` when done, note commit hash
4. New discoveries → add notes, flag in FINDINGS.md
