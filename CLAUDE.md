# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`go-p2p` is the P2P networking layer for the Lethean network. Module path: `forge.lthn.ai/core/go-p2p`

## Prerequisites

Private dependencies (`Borg`, `Poindexter`, `Enchantrix`) are hosted on `forge.lthn.ai`. Required env:
```bash
GOPRIVATE=forge.lthn.ai
```
SSH key must be configured for `git@forge.lthn.ai:2223`. Push to `forge` remote only.

## Commands

```bash
go test ./...                    # Run all tests
go test -run TestName ./...      # Single test
go test -race ./...              # Race detector (required before any PR)
go test -short ./...             # Skip integration tests (they bind real TCP ports)
go test -cover ./node            # Coverage for a specific package
go test -bench . -benchmem ./... # Benchmarks with allocation tracking
go vet ./...                     # Static analysis
golangci-lint run ./...          # Linting
```

## Architecture

Three packages plus one subpackage:

```
node/        — P2P mesh: identity, transport, peers, protocol, workers, controller, dispatcher, bundles
node/levin/  — CryptoNote Levin binary protocol (standalone, no parent imports)
ueps/        — UEPS wire protocol (RFC-021): TLV packet builder and stream reader (stdlib only)
logging/     — Structured levelled logger with component scoping (stdlib only)
```

### Data flow

1. **Identity** (`identity.go`) — Ed25519 keypair via Borg STMF. Shared secrets derived via X25519 ECDH + SHA-256.
2. **Transport** (`transport.go`) — WebSocket server/client (gorilla/websocket). Handshake exchanges `NodeIdentity` + HMAC-SHA256 challenge-response. Post-handshake messages are Borg SMSG-encrypted. Includes deduplication (5-min TTL), rate limiting (token bucket: 100 burst/50 per sec), and MaxConns enforcement.
3. **Dispatcher** (`dispatcher.go`) — Routes verified UEPS packets to intent handlers. Threat circuit breaker drops packets with `ThreatScore > 50,000` before routing.
4. **Controller** (`controller.go`) — Issues requests to remote peers using a pending-map pattern (`map[string]chan *Message`). Auto-connects to peers on demand.
5. **Worker** (`worker.go`) — Handles incoming commands via `MinerManager` and `ProfileManager` interfaces.
6. **Peer Registry** (`peer.go`) — KD-tree peer selection across 4 dimensions (latency, hops, geography, reliability). Persistence uses atomic rename with 5-second debounced writes.
7. **Levin** (`node/levin/`) — CryptoNote binary protocol: header parsing, portable storage decode, varint encoding. Completely standalone subpackage.

### Key interfaces

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

### Dependency codenames

- **Borg** — STMF crypto (key generation), SMSG (symmetric encryption), TIM (deployment bundle encryption)
- **Poindexter** — KD-tree for peer selection
- **Enchantrix** — Secure environment (indirect, via Borg)

## Coding Standards

- UK English (colour, organisation, centre, behaviour, recognise)
- All parameters and return types explicitly annotated
- Tests use `testify` assert/require; table-driven subtests with `t.Run()`
- Test name suffixes: `_Good` (happy path), `_Bad` (expected errors), `_Ugly` (panic/edge cases)
- Licence: EUPL-1.2 — new files need `// SPDX-License-Identifier: EUPL-1.2`
- Security-first: do not weaken HMAC, challenge-response, Zip Slip defence, or rate limiting
- Use `logging` package only — no `fmt.Println` or `log.Printf` in library code
- Hot-path debug logging uses sampling pattern: `if counter.Add(1)%interval == 0`

### Transport test helper

Tests needing live WebSocket endpoints use the reusable helper:
```go
tp := setupTestTransportPair(t)        // creates two transports on ephemeral ports
pc := tp.connectClient(t)              // performs real handshake, returns *PeerConnection
// tp.Server, tp.Client, tp.ServerNode, tp.ClientNode, tp.ServerReg, tp.ClientReg
```
Cleanup is automatic via `t.Cleanup`.

## Commit Format

```
type(scope): description

Co-Authored-By: Virgil <virgil@lethean.io>
```

Types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`, `ci`
Scopes: `node`, `ueps`, `logging`, `transport`, `peer`, `dispatcher`, `identity`, `bundle`, `controller`, `levin`

## Documentation

- `docs/architecture.md` — full package and component reference
- `docs/development.md` — build, test, benchmark, standards guide
- `docs/history.md` — completed phases, known limitations, bugs fixed
