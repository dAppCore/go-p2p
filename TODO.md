# TODO.md — go-p2p Task Queue

Dispatched from core/go orchestration. Pick up tasks in phase order.

---

## Phase 1: UEPS Wire Protocol Tests (CRITICAL — 0% coverage)

The UEPS packet builder and reader implement HMAC-SHA256 signed TLV frames. Zero tests exist. This is crypto code — it must be tested.

- [ ] **PacketBuilder round-trip** — Build packet with known fields, marshal+sign, then ReadAndVerify, assert all header fields match and payload is intact.
- [ ] **HMAC verification** — Tamper with payload byte after signing, verify ReadAndVerify returns integrity error. Do same for header tampering.
- [ ] **Wrong shared secret** — Sign with key A, verify with key B, expect HMAC mismatch.
- [ ] **Empty payload** — Payload=nil or []byte{}, should produce valid signed packet.
- [ ] **Max ThreatScore boundary** — ThreatScore=65535 (uint16 max), verify serialisation round-trips correctly.
- [ ] **Missing HMAC tag** — Craft packet without 0x06 tag, expect "missing HMAC" error from reader.
- [ ] **TLV value too large** — Value >255 bytes, expect writeTLV error.
- [ ] **Truncated packet** — Short read / EOF mid-TLV, expect io error.
- [ ] **Unknown TLV tag** — Insert unknown tag between header TLVs and HMAC, verify reader skips it but includes in signature check.

## Phase 2: Transport Tests (0 tests, 934 lines)

Transport is the encrypted WebSocket layer. Tests need real WebSocket connections via httptest.NewServer.

- [ ] **Test pair setup helper** — Create reusable helper that spins up two identities, registries (open auth), transports on random ports. This helper underpins all transport tests.
- [ ] **Full handshake** — Client connects to server, challenge-response completes, shared secret derived, both sides have connection.
- [ ] **Handshake rejection: wrong protocol version** — Peer with incompatible version gets rejection message before disconnect.
- [ ] **Handshake rejection: allowlist** — Peer not in allowlist gets "not authorized" rejection.
- [ ] **Encrypted message round-trip** — Send message from A to B via SMSG encryption, verify decrypt and content match.
- [ ] **Message deduplication** — Send message with same ID twice, second is dropped silently.
- [ ] **Rate limiting** — Burst >100 messages from one peer, verify messages dropped after token bucket empties.
- [ ] **MaxConns enforcement** — Fill MaxConns, next connection gets 503 rejection.
- [ ] **Keepalive timeout** — No activity beyond PingInterval+PongTimeout, connection cleaned up.
- [ ] **Graceful close** — GracefulClose sends disconnect message (MsgDisconnect) before closing.
- [ ] **Concurrent sends** — Multiple goroutines sending on same connection, no races (writeMu protects).

## Phase 3: Controller Tests (0 tests, 327 lines)

Controller wraps transport for request-response patterns. Test over a real transport pair from Phase 2.

- [ ] **Request-response correlation** — Send request, worker replies with ReplyTo set, controller matches correctly.
- [ ] **Request timeout** — No response within deadline, returns timeout error.
- [ ] **Auto-connect** — Peer not connected, controller auto-connects via transport before sending.
- [ ] **GetAllStats** — Multiple connected peers, verify parallel stat collection completes.
- [ ] **PingPeer RTT** — Send ping, receive pong, RTT calculated and peer metrics updated.
- [ ] **Concurrent requests** — Multiple requests in flight to different peers, correct correlation.
- [ ] **Dead peer cleanup** — Response channel cleaned up after timeout (no goroutine/memory leak).

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
