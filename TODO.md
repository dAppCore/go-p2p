# TODO

## High Priority — Test Coverage (currently 42%)

- [ ] **UEPS packet tests** — Zero tests for wire protocol. Need: builder round-trip, HMAC verification, malformed packet rejection, empty payload, oversized payload, max ThreatScore boundary.
- [ ] **Transport tests** — 934 lines untested. Need: WebSocket handshake (accept + reject), SMSG encryption round-trip, connection lifecycle, keepalive timeout, rate limiting, deduplication, protocol version mismatch.
- [ ] **Controller tests** — 327 lines untested. Need: request-response correlation, timeout handling, auto-connect, concurrent requests, GetAllStats parallel execution.

## Medium Priority — Coverage Target 70%+

- [ ] **Dispatcher implementation** — Currently a commented-out stub. Implement UEPS packet routing with threat circuit breaker (drop ThreatScore > 50000) and intent-based dispatch.
- [ ] **Integration test** — Full node-to-node handshake over localhost WebSocket with encrypted message exchange.
- [ ] **Benchmarks** — Peer scoring (KD-tree), UEPS marshal/unmarshal, identity key generation, message serialisation.
- [ ] **bufpool.go tests** — Buffer reuse verification, large buffer handling.

## Low Priority

- [ ] **Logging package tests** — Simple but should have coverage for completeness.
- [ ] **Peer discovery** — Currently manual peer registration. Add mDNS or DHT-based discovery.
- [ ] **Connection pooling** — Transport creates fresh connections; add pool for controller reuse.
- [ ] **Error recovery tests** — Handshake timeouts, protocol version mismatch, allowlist rejection, connection drop/reconnect.
