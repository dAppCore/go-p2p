# Session Brief: core/go-p2p

**Repo**: `forge.lthn.ai/core/go-p2p` (clone at `/tmp/core-go-p2p`)
**Module**: `forge.lthn.ai/core/go-p2p`
**Status**: 16 Go files, ~2,500 LOC, node tests PASS (42% coverage), ueps has NO TESTS
**Wiki**: https://forge.lthn.ai/core/go-p2p/wiki (6 pages)

## What This Is

P2P networking layer for the Lethean network. Three packages:

### node/ — P2P Mesh (14 files)
- **Identity**: Ed25519 keypair generation, PEM serialisation, challenge-response auth
- **Transport**: Encrypted WebSocket connections via gorilla/websocket + Borg (encrypted blob storage)
- **Peers**: Registry with scoring, persistence, auth modes (open/allowlist), name validation
- **Messages**: Typed protocol messages (handshake, ping, stats, miner control, deploy, logs)
- **Protocol**: Response handler with validation and typed parsing
- **Worker**: Command handler (ping, stats, miner start/stop, deploy profiles, get logs)
- **Dispatcher**: UEPS packet routing skeleton with threat circuit breaker
- **Controller**: Remote node operations (connect, command, disconnect)
- **Bundle**: Service factory for Core framework DI registration

### ueps/ — Wire Protocol (2 files, NO TESTS)
- **PacketBuilder**: Constructs signed UEPS frames with TLV encoding
- **ReadAndVerify**: Parses and verifies HMAC-SHA256 integrity
- TLV tags: 0x01-0x05 (header fields), 0x06 (HMAC), 0xFF (payload marker)
- Header: Version, CurrentLayer, TargetLayer, IntentID, ThreatScore

### logging/ — Structured Logger (1 file)
- Simple levelled logger (INFO/WARN/ERROR/DEBUG) with key-value pairs

## Current State

| Area | Status |
|------|--------|
| node/ tests | PASS — 42% statement coverage |
| ueps/ tests | NONE — zero test files |
| logging/ tests | NONE |
| go vet | Clean |
| TODOs/FIXMEs | None found |
| Identity (Ed25519) | Well tested — keypair, challenge-response, deterministic sigs |
| PeerRegistry | Well tested — add/remove, scoring, persistence, auth modes, name validation |
| Messages | Well tested — all 15 message types, serialisation, error codes |
| Worker | Well tested — ping, stats, miner, deploy, logs handlers |
| Transport | NOT tested — WebSocket + Borg encryption |
| Controller | NOT tested — remote node operations |
| Dispatcher | NOT tested — UEPS routing skeleton |

## Dependencies

- `github.com/Snider/Borg` v0.2.0 (encrypted blob storage)
- `github.com/Snider/Enchantrix` v0.0.2 (secure environment)
- `github.com/Snider/Poindexter` (secure pointer)
- `github.com/gorilla/websocket` v1.5.3
- `github.com/google/uuid` v1.6.0
- `github.com/ProtonMail/go-crypto` v1.3.0
- `github.com/adrg/xdg` v0.5.3
- `github.com/stretchr/testify` v1.11.1
- `golang.org/x/crypto` v0.45.0

## Priority Work

### High (coverage gaps)
1. **UEPS tests** — Zero tests for the wire protocol. This is the consent-gated TLV protocol from RFC-021. Need: builder round-trip, HMAC verification, malformed packet rejection, boundary conditions (max ThreatScore, empty payload, oversized payload).
2. **Transport tests** — WebSocket connection, Borg encryption handshake, reconnection logic.
3. **Controller tests** — Connect/command/disconnect flow.
4. **Dispatcher tests** — UEPS routing, threat circuit breaker (ThreatScore > 50000 drops).

### Medium (hardening)
5. **Increase node/ coverage** from 42% to 70%+ — focus on transport.go, controller.go, dispatcher.go
6. **Benchmarks** — Peer scoring, UEPS marshal/unmarshal, identity key generation
7. **Integration test** — Full node-to-node handshake over localhost WebSocket

### Low (completeness)
8. **Logging tests** — Simple but should have coverage
9. **Peer discovery** — Currently manual. Add mDNS or DHT discovery
10. **Connection pooling** — Transport creates fresh connections; add pool for controller

## File Map

```
/tmp/core-go-p2p/
├── node/
│   ├── bundle.go          + bundle_test.go       — Core DI factory
│   ├── identity.go        + identity_test.go     — Ed25519 keypair, PEM, challenge-response
│   ├── message.go         + message_test.go      — Protocol message types
│   ├── peer.go            + peer_test.go         — Registry, scoring, auth
│   ├── protocol.go        + protocol_test.go     — Response validation, typed parsing
│   ├── worker.go          + worker_test.go       — Command handlers
│   ├── transport.go       (NO TEST)              — WebSocket + Borg encryption
│   ├── controller.go      (NO TEST)              — Remote node operations
│   ├── dispatcher.go      (NO TEST)              — UEPS routing skeleton
│   └── logging.go                                — Package-level logger setup
├── ueps/
│   ├── ueps.go            (NO TEST)              — PacketBuilder, ReadAndVerify, TLV
│   └── types.go           (NO TEST)              — UEPSHeader, ParsedPacket, intent IDs
├── logging/
│   └── logger.go          (NO TEST)              — Levelled structured logger
├── go.mod
└── go.sum
```

## Key Interfaces

```go
// node/message.go — 15 message types
const (
    MsgHandshake    MsgHandshakeAck   MsgPing        MsgPong
    MsgDisconnect   MsgGetStats       MsgStats       MsgStartMiner
    MsgStopMiner    MsgMinerAck       MsgDeploy      MsgDeployAck
    MsgGetLogs      MsgLogs           MsgError
)

// ueps/types.go — UEPS header
type UEPSHeader struct {
    Version      uint8   // 0x09
    CurrentLayer uint8
    TargetLayer  uint8
    IntentID     uint8   // 0x01=Handshake, 0x20=Compute, 0x30=Rehab, 0xFF=Extended
    ThreatScore  uint16
}
```

## Conventions

- UK English
- Tests: testify assert/require
- Licence: EUPL-1.2
- Lethean codenames: Borg (Secure/Blob), Poindexter (Secure/Pointer), Enchantrix (Secure/Environment)
