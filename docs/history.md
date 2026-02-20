# Project History — go-p2p

## Phases

### Phase 1 — UEPS Wire Protocol Tests

Commit `2bc53ba`. Coverage: ueps/ 88.5%.

Implemented the complete test suite for the UEPS binary framing layer. Tests covered every aspect of the TLV encoding and HMAC-SHA256 signing:

- PacketBuilder round-trip: basic, binary payload, elevated threat score, large payload
- HMAC verification: payload tampering detected, header tampering detected, wrong shared secret detected
- Boundary conditions: nil payload, empty slice payload, `uint16` max ThreatScore (65,535), TLV value exceeding 255 bytes (`writeTLV` error path)
- Stream robustness: truncated packets detected at multiple cut points (EOF mid-tag, mid-length, mid-value), missing HMAC tag, unknown TLV tags skipped and included in signed data

The 11.5% gap from 100% coverage is the reader's `io.ReadAll` error path, which requires a contrived broken `io.Reader` to exercise.

### Phase 2 — Transport Tests

Commit `3ee5553`. Coverage: node/ 42% to 63.5%.

Implemented transport layer tests with real WebSocket connections (no mocks). A reusable `setupTestTransportPair` helper creates two live transports on ephemeral ports and performs identity generation.

Tests covered:
- Full handshake: challenge-response completes, 32-byte shared secret derived
- Handshake rejection: incompatible protocol version (rejection message sent before disconnect)
- Handshake rejection: allowlist mode, peer not authorised
- Encrypted message round-trip: SMSG encrypt on one side, decrypt on other
- Message deduplication: duplicate UUID dropped silently
- Rate limiting: burst of more than 100 messages, subsequent drops after token bucket empties
- MaxConns enforcement: 503 HTTP rejection when limit is reached
- Keepalive timeout: connection cleaned up after `PingInterval + PongTimeout` elapses
- Graceful close: `MsgDisconnect` sent before underlying WebSocket close
- Concurrent sends: no data races under `go test -race` (`writeMu` protects all writes)

### Phase 3 — Controller Tests

Commit `33eda7b`. Coverage: node/ 63.5% to 72.1%. 14 test functions.

Also fixed bug P2P-RACE-1 (see Known Issues).

Tests covered:
- Request-response correlation: message sent, worker replies with `ReplyTo` set, controller matches by ID
- Request timeout: no response within deadline, `sendRequest` returns timeout error, pending channel cleaned up
- Auto-connect: peer not yet connected, controller calls `transport.Connect` transparently
- GetAllStats: multiple connected peers, parallel stat collection, all results collected
- PingPeer RTT: ping sent, pong received, RTT calculated in milliseconds, peer metrics updated in registry
- Concurrent requests: multiple in-flight requests to different peers, correct correlation under load
- Dead peer cleanup: response channel closed and removed from pending map after timeout (no goroutine leak)

### Phase 4 — Dispatcher Implementation

Commit `a60dfdf`. Coverage: dispatcher.go 100%.

Replaced the dispatcher stub with a complete implementation. 10 test functions, 17 subtests.

Design decisions recorded at the time:

1. `IntentHandler` as a `func` type rather than an interface, matching the `MessageHandler` pattern already used in `transport.go`. Lighter weight for a single-method contract.
2. Sentinel errors (`ErrThreatScoreExceeded`, `ErrUnknownIntent`, `ErrNilPacket`) rather than silent drops. Callers can inspect outcomes; the dispatcher still logs at WARN level regardless.
3. Threat check occurs before intent routing. A high-threat packet with an unknown intent returns `ErrThreatScoreExceeded`, not `ErrUnknownIntent`. The circuit breaker is the first line of defence.
4. Threshold fixed at 50,000 (a constant, not configurable) to match the original stub specification. The value sits at approximately 76% of `uint16` max.
5. `sync.RWMutex` for the handler map. Registration is infrequent (write lock); dispatch is read-heavy (read lock).

Tests covered: register/dispatch, threat boundary conditions (at threshold, above threshold, `uint16` max, zero), unknown intent, multi-handler routing, nil packet, empty payload, concurrent dispatch (50 goroutines), concurrent register-and-dispatch, handler replacement, threat-before-routing ordering, intent constant value verification.

### Phase 5 — Integration Tests and Benchmarks

Coverage: race-free under `go test -race`.

Three integration tests (`TestIntegration_*`) exercise the full stack end-to-end:

- `TestIntegration_TwoNodeHandshakeAndMessage`: two nodes on localhost, identity creation, handshake, encrypted message exchange, controller ping/pong with RTT measurement, UEPS packet routing via dispatcher, threat circuit breaker verification, graceful shutdown with disconnect message.
- `TestIntegration_SharedSecretAgreement`: verifies that two independently created nodes derive identical 32-byte shared secrets via X25519 ECDH (fundamental correctness property).
- `TestIntegration_GetRemoteStats_EndToEnd`: full stats retrieval across a real WebSocket connection with worker and controller wired together.

13 benchmark functions across `node/` and `ueps/`:
- Identity operations: keygen, shared secret derivation, challenge generation, challenge sign+verify
- Message operations: serialise
- Transport operations: SMSG encrypt+decrypt
- Peer registry: KD-tree select, KD-tree rebuild
- UEPS: marshal, read+verify
- Buffer pool: get/put (zero allocations confirmed)

9 buffer pool tests (`bufpool_test.go`): get/put round-trip, buffer reuse verification, large buffer eviction (buffers exceeding 64 KB are not returned to the pool), concurrent get/put (100 goroutines × 50 iterations), buffer independence, `MarshalJSON` correctness for 7 payload types, independent copy verification, HTML escaping disabled, concurrent `MarshalJSON`.

## Known Limitations

### UEPS 0xFF Payload Not Self-Delimiting

The `TagPayload` (0xFF) field carries no length prefix. `ReadAndVerify` calls `io.ReadAll` on the remaining stream, which means the packet format relies on external TCP framing to delimit the packet boundary. The enclosing transport must provide a length-prefixed frame before calling `ReadAndVerify`. This is noted in comments in both `packet.go` and `reader.go` but no solution is implemented.

Consequence: UEPS packets cannot be chained in a raw stream without an outer framing protocol. The current WebSocket transport encapsulates each UEPS frame in a single WebSocket message, which provides the necessary boundary implicitly.

### No Resource Cleanup on Some Error Paths

`transport.handleWSUpgrade` does not clean up on handshake timeout (the `pendingConns` counter is decremented correctly via `defer`, but the underlying WebSocket connection may linger briefly before the read deadline fires). `transport.Connect` does not clean up the temporary connection object on handshake failure (the raw WebSocket `conn` is closed, but there is no registry or metrics cleanup for the partially constructed `PeerConnection`).

These are low-severity gaps. They do not cause goroutine leaks under the current implementation because the connection's read loop is not started until after a successful handshake.

### Controller Race (Resolved)

The originally identified risk — that `transport.OnMessage(c.handleResponse)` is called during `NewController` initialisation and a message arriving before the pending map is ready could cause a panic — was confirmed to be a false alarm. The pending map is initialised in `NewController` before `OnMessage` is called, and `handleResponse` uses a mutex on all map access. No panic is possible.

## Bugs Fixed

### P2P-RACE-1 — GracefulClose Data Race (Phase 3)

`GracefulClose` previously called `pc.Conn.SetWriteDeadline()` outside of `writeMu`, racing with concurrent `Send()` calls that also set the write deadline. Detected by `go test -race`.

Fix: removed the bare `SetWriteDeadline` call from `GracefulClose`. The method now relies entirely on `Send()`, which manages write deadlines under `writeMu`. This is documented in a comment in `transport.go` to prevent the pattern from being reintroduced.

## Wiki Corrections (19 February 2026)

Three wiki inconsistencies were identified and corrected:

- The Node-Identity page stated `PublicKey` is hex-encoded. The code uses base64 (`identity.go:63`).
- The Protocol-Messages page used a `Sender` field. The code uses `From` and `To` (`message.go:66-67`).
- The Peer-Discovery page stated `Score` is in the range 0.0–1.0. The code uses a float64 range of 0–100 (`peer.go:31`).
