# CLAUDE.md

## Project

`go-p2p` is the P2P networking layer for the Lethean network. Module path: `forge.lthn.ai/core/go-p2p`

## Commands

```bash
go test ./...                    # Run all tests
go test -run TestName ./...      # Single test
go test -cover ./node            # Coverage for node package
go test -bench . ./...           # Benchmarks
go vet ./...                     # Static analysis
```

## Architecture

Three packages:

### node/ — P2P Mesh
- **identity.go**: Ed25519 keypair, PEM serialisation, X25519 ECDH, challenge-response auth
- **transport.go**: Encrypted WebSocket (gorilla/websocket + Borg SMSG), handshake, keepalive, dedup, rate limiting
- **peer.go**: Registry with KD-tree scoring (Poindexter), persistence, auth modes (open/allowlist)
- **message.go**: 15 typed protocol messages (handshake, ping, stats, miner, deploy, logs, error)
- **protocol.go**: Response handler with validation and typed parsing
- **worker.go**: Command handlers (ping, stats, miner start/stop, deploy, logs)
- **controller.go**: Remote node operations (connect, command, disconnect)
- **dispatcher.go**: UEPS packet routing skeleton (STUB — needs implementation)
- **bundle.go**: TIM encryption, tarball extraction with Zip Slip defence

### ueps/ — Wire Protocol (RFC-021)
- **packet.go**: PacketBuilder with TLV encoding and HMAC-SHA256 signing
- **reader.go**: Stream parser with integrity verification
- TLV tags: 0x01-0x05 (header), 0x06 (HMAC), 0xFF (payload marker)
- Header: Version (0x09), CurrentLayer, TargetLayer, IntentID, ThreatScore

### logging/ — Structured Logger
- Levelled (DEBUG/INFO/WARN/ERROR) with key-value pairs and component scoping

## Dependencies

- `github.com/Snider/Borg` — STMF crypto, SMSG encryption, TIM
- `github.com/Snider/Poindexter` — KD-tree for peer selection
- `github.com/Snider/Enchantrix` — Secure environment (via Borg)
- `github.com/gorilla/websocket` — WebSocket transport
- `github.com/google/uuid` — Peer/message IDs
- Lethean codenames: Borg (Secure/Blob), Poindexter (Secure/Pointer), Enchantrix (Secure/Environment)

## Coding Standards

- UK English (colour, organisation, centre)
- All types annotated
- Tests use `testify` assert/require
- Licence: EUPL-1.2
- Security-first: HMAC on all wire traffic, challenge-response auth, Zip Slip defence, rate limiting

## Test Conventions

Use table-driven subtests with `t.Run()`.

## Key Interfaces

```go
// MinerManager — decoupled miner control (worker.go)
type MinerManager interface {
    StartMiner(config map[string]any) error
    StopMiner(id string) error
    GetStats() map[string]any
    GetLogs(id string, lines int) ([]string, error)
}

// ProfileManager — deployment profiles (worker.go)
type ProfileManager interface {
    ApplyProfile(name string, data []byte) error
}
```
