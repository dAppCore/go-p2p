---
title: go-p2p Overview
description: P2P mesh networking layer for the Lethean network.
---

# go-p2p

P2P networking layer for the Lethean network. Encrypted WebSocket mesh with UEPS wire protocol.

**Module:** `forge.lthn.ai/core/go-p2p`
**Go:** 1.26
**Licence:** EUPL-1.2

## Package Structure

```
go-p2p/
├── node/        P2P mesh: identity, transport, peers, protocol, controller, dispatcher
│   └── levin/   Levin binary protocol (header, storage, varint, connection)
├── ueps/        UEPS wire protocol (RFC-021): TLV packet builder and stream reader
└── logging/     Structured levelled logger with component scoping
```

## What Each Piece Does

| Component | File(s) | Purpose |
|-----------|---------|---------|
| [Identity](identity.md) | `identity.go` | X25519 keypair, node ID derivation, HMAC-SHA256 challenge-response |
| [Transport](transport.md) | `transport.go` | Encrypted WebSocket connections, SMSG encryption, rate limiting |
| [Discovery](discovery.md) | `peer.go` | Peer registry, KD-tree selection, score tracking, allowlist auth |
| [UEPS](ueps.md) | `ueps/packet.go`, `ueps/reader.go` | TLV wire protocol with HMAC integrity (RFC-021) |
| [Routing](routing.md) | `dispatcher.go` | Intent-based packet routing with threat circuit breaker |
| [TIM Bundles](tim.md) | `bundle.go` | Encrypted deployment bundles, tar extraction with Zip Slip defence |
| Messages | `message.go` | Message envelope, payload types, protocol version negotiation |
| Protocol | `protocol.go` | Response validation, structured error handling |
| Controller | `controller.go` | Request-response correlation, remote peer operations |
| Worker | `worker.go` | Incoming message dispatch, miner/profile management interfaces |
| Buffer Pool | `bufpool.go` | `sync.Pool`-backed JSON encoding for hot paths |

## Dependencies

| Module | Purpose |
|--------|---------|
| `forge.lthn.ai/Snider/Borg` | STMF crypto (keypairs), SMSG encryption, TIM bundle format |
| `forge.lthn.ai/Snider/Poindexter` | KD-tree peer scoring and nearest-neighbour selection |
| `github.com/gorilla/websocket` | WebSocket transport |
| `github.com/google/uuid` | Message and peer ID generation |
| `github.com/adrg/xdg` | XDG base directory paths for key and config storage |

## Message Protocol

Every message is a JSON-encoded `Message` struct transported over WebSocket. After handshake, all messages are SMSG-encrypted using the X25519-derived shared secret.

```go
type Message struct {
    ID        string          `json:"id"`                  // UUID v4
    Type      MessageType     `json:"type"`                // Determines payload interpretation
    From      string          `json:"from"`                // Sender node ID
    To        string          `json:"to"`                  // Recipient node ID
    Timestamp time.Time       `json:"ts"`
    Payload   json.RawMessage `json:"payload"`             // Type-specific JSON
    ReplyTo   string          `json:"replyTo,omitempty"`   // For request-response correlation
}
```

### Message Types

| Category | Types |
|----------|-------|
| Connection | `handshake`, `handshake_ack`, `ping`, `pong`, `disconnect` |
| Operations | `get_stats`, `stats`, `start_miner`, `stop_miner`, `miner_ack` |
| Deployment | `deploy`, `deploy_ack` |
| Logs | `get_logs`, `logs` |
| Error | `error` (codes 1000--1005) |

## Node Roles

```go
const (
    RoleController NodeRole = "controller"  // Orchestrates work distribution
    RoleWorker     NodeRole = "worker"      // Executes compute tasks
    RoleDual       NodeRole = "dual"        // Both controller and worker
)
```

## Architecture Layers

The stack has two distinct protocol layers:

1. **UEPS (low-level)** -- Binary TLV wire protocol with HMAC-SHA256 integrity, intent routing, and threat scoring. Operates beneath the mesh layer. See [ueps.md](ueps.md).

2. **Node mesh (high-level)** -- JSON-over-WebSocket with SMSG encryption. Handles identity, peer management, controller/worker operations, and deployment bundles.

The dispatcher bridges the two layers, routing verified UEPS packets to registered intent handlers whilst enforcing the threat circuit breaker.
