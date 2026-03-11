---
title: Encrypted WebSocket Transport
description: SMSG-encrypted WebSocket connections with HMAC handshake, rate limiting, and message deduplication.
---

# Encrypted WebSocket Transport

The `Transport` manages encrypted WebSocket connections between nodes. After an HMAC-SHA256 challenge-response handshake, all messages are encrypted using SMSG (from the Borg library) with the X25519-derived shared secret.

## Configuration

```go
type TransportConfig struct {
    ListenAddr     string        // ":9091" default
    WSPath         string        // "/ws" -- WebSocket endpoint path
    TLSCertPath    string        // Optional TLS for wss://
    TLSKeyPath     string
    MaxConns       int           // Maximum concurrent connections (default 100)
    MaxMessageSize int64         // Maximum message size in bytes (default 1MB)
    PingInterval   time.Duration // Keepalive interval (default 30s)
    PongTimeout    time.Duration // Pong wait timeout (default 10s)
}
```

Sensible defaults via `DefaultTransportConfig()`:

```go
cfg := node.DefaultTransportConfig()
// ListenAddr: ":9091", WSPath: "/ws", MaxConns: 100
// MaxMessageSize: 1MB, PingInterval: 30s, PongTimeout: 10s
```

## Creating and Starting

```go
transport := node.NewTransport(nodeManager, peerRegistry, cfg)

// Set message handler before Start() to avoid races
transport.OnMessage(func(conn *node.PeerConnection, msg *node.Message) {
    // Handle incoming messages
})

err := transport.Start()
```

## TLS Hardening

When `TLSCertPath` and `TLSKeyPath` are set, the transport uses TLS with hardened settings:

- Minimum TLS 1.2
- Curve preferences: X25519, P-256
- AEAD cipher suites only (GCM and ChaCha20-Poly1305)

## Connection Handshake

The handshake sequence establishes identity and derives the encryption key:

1. **Initiator** sends `MsgHandshake` containing its `NodeIdentity`, a 32-byte random challenge, and the protocol version (`"1.0"`).
2. **Responder** derives the shared secret via X25519 ECDH, checks the protocol version is supported, verifies the peer against the allowlist (if `PeerAuthAllowlist` mode), signs the challenge with HMAC-SHA256, and sends `MsgHandshakeAck` with its own identity and the challenge response.
3. **Initiator** derives the shared secret, verifies the HMAC response, and stores the shared secret.
4. All subsequent messages are SMSG-encrypted.

Both the handshake and acknowledgement are sent unencrypted -- they carry the public keys needed to derive the shared secret. A 10-second timeout prevents slow or malicious peers from blocking the handshake.

### Rejection

The responder rejects connections with a `HandshakeAckPayload` where `Accepted: false` and a `Reason` string for:

- Incompatible protocol version
- Peer not on the allowlist (when in allowlist mode)

## Message Encryption

After handshake, messages are encrypted with SMSG using the shared secret:

```
Send path:  Message -> JSON (pooled buffer) -> SMSG encrypt -> WebSocket binary frame
Recv path:  WebSocket frame -> SMSG decrypt -> JSON unmarshal -> Message
```

The shared secret is base64-encoded before use as the SMSG password. This is handled internally by `encryptMessage()` and `decryptMessage()`.

## PeerConnection

Each active connection is wrapped in a `PeerConnection`:

```go
type PeerConnection struct {
    Peer         *Peer              // Remote peer identity
    Conn         *websocket.Conn    // Underlying WebSocket
    SharedSecret []byte             // From X25519 ECDH
    LastActivity time.Time
}
```

### Sending Messages

```go
err := peerConn.Send(msg)
```

`Send()` serialises the message to JSON, encrypts it with SMSG, sets a 10-second write deadline, and writes as a binary WebSocket frame. A `writeMu` mutex serialises concurrent writes.

### Graceful Close

```go
err := peerConn.GracefulClose("shutting down", node.DisconnectShutdown)
```

Sends a `disconnect` message (best-effort) before closing the connection. Uses `sync.Once` to ensure the connection is only closed once.

### Disconnect Codes

```go
const (
    DisconnectNormal      = 1000 // Normal closure
    DisconnectGoingAway   = 1001 // Server/peer going away
    DisconnectProtocolErr = 1002 // Protocol error
    DisconnectTimeout     = 1003 // Idle timeout
    DisconnectShutdown    = 1004 // Server shutdown
)
```

## Incoming Connections

The transport exposes an HTTP handler at the configured `WSPath` that upgrades to WebSocket. Origin checks restrict browser clients to `localhost`, `127.0.0.1`, and `::1`; non-browser clients (no `Origin` header) are allowed.

The `MaxConns` limit is enforced before the WebSocket upgrade, counting both established and pending (mid-handshake) connections. Excess connections receive HTTP 503.

## Message Deduplication

`MessageDeduplicator` prevents duplicate message processing (amplification attack mitigation):

- Tracks message IDs with a configurable TTL (default 5 minutes)
- Checked after decryption, before handler dispatch
- Background cleanup runs every minute

## Rate Limiting

Each `PeerConnection` has a `PeerRateLimiter` implementing a token-bucket algorithm:

- **Burst:** 100 messages
- **Refill:** 50 tokens per second

Messages exceeding the rate limit are silently dropped with a warning log. This prevents a single peer from overwhelming the node.

## Keepalive

A background goroutine per connection sends `MsgPing` at the configured `PingInterval`. If no activity is observed within `PingInterval + PongTimeout`, the connection is closed and removed.

The read loop also sets a deadline of `PingInterval + PongTimeout` on each read, preventing indefinitely blocked reads on unresponsive connections.

## Lifecycle

```go
// Start listening and accepting connections
err := transport.Start()

// Connect to a known peer (triggers handshake)
pc, err := transport.Connect(peer)

// Send to a specific peer
err = transport.Send(peerID, msg)

// Broadcast to all connected peers (excludes sender)
err = transport.Broadcast(msg)

// Query connections
count := transport.ConnectedPeers()
conn := transport.GetConnection(peerID)

// Iterate over all connections
for pc := range transport.Connections() {
    // ...
}

// Graceful shutdown (sends disconnect to all peers, waits for goroutines)
err = transport.Stop()
```

## Buffer Pool

JSON encoding in hot paths uses `sync.Pool`-backed byte buffers (`bufpool.go`). The `MarshalJSON()` function:

- Uses pooled buffers (initial capacity 1024 bytes)
- Disables HTML escaping
- Returns a copy of the encoded bytes (safe after function return)
- Discards buffers exceeding 64KB to prevent pool bloat
