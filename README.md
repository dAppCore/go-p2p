[![Go Reference](https://pkg.go.dev/badge/forge.lthn.ai/core/go-p2p.svg)](https://pkg.go.dev/forge.lthn.ai/core/go-p2p)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](go.mod)

# go-p2p

P2P mesh networking layer for the Lethean network. Provides Ed25519 node identity, an encrypted WebSocket transport with HMAC-SHA256 challenge-response handshake, KD-tree peer selection across four dimensions (latency, hops, geography, reliability score), UEPS wire protocol (RFC-021) TLV packet builder and reader, UEPS intent routing with a threat circuit breaker, and TIM deployment bundle encryption with Zip Slip and decompression-bomb defences.

**Module**: `forge.lthn.ai/core/go-p2p`
**Licence**: EUPL-1.2
**Language**: Go 1.25

## Quick Start

```go
import (
    "forge.lthn.ai/core/go-p2p/node"
    "forge.lthn.ai/core/go-p2p/ueps"
)

// Start a P2P node
identity, _ := node.LoadOrCreateIdentity()
transport := node.NewTransport(identity, node.TransportConfig{ListenAddr: ":9091"})
transport.Start(ctx)

// Build a UEPS packet
pkt, _ := ueps.NewBuilder(ueps.IntentCompute, payload).MarshalAndSign(sharedSecret)
```

## Documentation

- [Architecture](docs/architecture.md) — node identity, transport, peer registry, UEPS protocol, dispatcher
- [Development Guide](docs/development.md) — building, testing, benchmarks, security rules
- [Project History](docs/history.md) — completed phases and known limitations

## Build & Test

```bash
go test ./...
go test -race ./...
go test -short ./...   # skip integration tests
go build ./...
```

## Licence

European Union Public Licence 1.2 — see [LICENCE](LICENCE) for details.
