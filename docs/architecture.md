# Architecture — go-p2p

`go-p2p` is the P2P networking layer for the Lethean network. Module path: `forge.lthn.ai/core/go-p2p`.

## Package Structure

Three packages compose the library:

```
go-p2p/
├── node/       — P2P mesh: identity, transport, peers, protocol, workers, controller, dispatcher
├── ueps/       — UEPS wire protocol (RFC-021): packet builder and stream reader
└── logging/    — Structured levelled logger with component scoping
```

## node/ — P2P Mesh

### identity.go — Node Identity

Each node holds an Ed25519 keypair generated via Borg STMF (X25519 curve). The private key is stored at `~/.local/share/lethean-desktop/node/private.key` (mode 0600) and the public identity JSON at `~/.config/lethean-desktop/node.json`.

`NodeIdentity` carries:
- `ID` — 32-character hex string derived from SHA-256 of the public key (first 16 bytes)
- `PublicKey` — base64-encoded X25519 public key
- `Role` — `controller`, `worker`, or `dual`

Shared secrets are derived via X25519 ECDH and then hashed with SHA-256, producing a 32-byte symmetric key used for all subsequent SMSG encryption on that connection.

Challenge-response authentication uses HMAC-SHA256 over a 32-byte random challenge. The challenger generates the nonce, the responder signs it with the shared secret, and the challenger verifies with `hmac.Equal` to prevent timing attacks.

### transport.go — Encrypted WebSocket Transport

The `Transport` manages a WebSocket server (gorilla/websocket) and outbound connections. All post-handshake messages are encrypted with Borg SMSG using the per-connection shared secret.

**Configuration** (`TransportConfig`):

| Field | Default | Purpose |
|-------|---------|---------|
| `ListenAddr` | `:9091` | HTTP bind address |
| `WSPath` | `/ws` | WebSocket endpoint |
| `MaxConns` | 100 | Maximum concurrent connections |
| `MaxMessageSize` | 1 MB | Read limit per message |
| `PingInterval` | 30 s | Keepalive ping period |
| `PongTimeout` | 10 s | Maximum time to wait for pong |

**TLS hardening**: When `TLSCertPath` and `TLSKeyPath` are set the server enforces TLS 1.2 minimum with a curated cipher suite (AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305) and curve preferences (X25519, P-256).

**Connection lifecycle**:

1. Client dials WebSocket, sends unencrypted `MsgHandshake` containing its `NodeIdentity`, a 32-byte random challenge, and the protocol version string.
2. Server checks `IsProtocolVersionSupported`, derives the shared secret from the client's public key, checks `IsPeerAllowed` (open or allowlist mode), and replies with `MsgHandshakeAck` containing its own identity and an HMAC-SHA256 signature of the challenge.
3. Client verifies the challenge response, stores the shared secret, and transitions to encrypted mode.
4. Subsequent messages are SMSG-encrypted binary WebSocket frames.

**Deduplication**: A `MessageDeduplicator` with a 5-minute TTL tracks seen message UUIDs. Duplicate messages arriving within the window are dropped silently, preventing amplification attacks.

**Rate limiting**: Each `PeerConnection` holds a `PeerRateLimiter` (token bucket: 100 burst, 50 tokens/second refill). Messages from rate-limited peers are dropped in the read loop.

**MaxConns enforcement**: The handler tracks `pendingConns` (atomic counter) during the handshake phase in addition to established connections, preventing races where a surge of simultaneous inbounds could exceed the limit.

**Keepalive**: A goroutine per connection ticks at `PingInterval`. If `LastActivity` has not been updated within `PingInterval + PongTimeout`, the connection is removed.

**Graceful close**: `GracefulClose` sends `MsgDisconnect` before closing the underlying WebSocket. Write deadlines are managed exclusively inside `Send()` under `writeMu` to prevent the race (P2P-RACE-1) where a bare `SetWriteDeadline` call could race with concurrent sends.

**Buffer pool**: `MarshalJSON` uses a `sync.Pool` of `bytes.Buffer` (initial capacity 1 KB, maximum pooled size 64 KB) to reduce allocation pressure in the message serialisation hot path. HTML escaping is disabled to match `json.Marshal` semantics.

### peer.go — Peer Registry with KD-Tree Selection

`PeerRegistry` maintains the set of known remote nodes and selects optimal peers via a 4-dimensional KD-tree (Poindexter library).

**Peer fields persisted**:
- `ID`, `Name`, `PublicKey`, `Address`, `Role`, `AddedAt`, `LastSeen`
- `PingMS`, `Hops`, `GeoKM`, `Score` (float64, 0–100)

**KD-tree dimensions** (lower is better in all axes):

| Dimension | Weight | Rationale |
|-----------|--------|-----------|
| `PingMS` | 1.0 | Latency dominates interactive performance |
| `Hops` | 0.7 | Network hop count (routing cost) |
| `GeoKM` | 0.2 | Geographic distance (minor factor) |
| `100 - Score` | 1.2 | Reliability (inverted so lower = better peer) |

`SelectOptimalPeer()` queries the tree for the point nearest to the origin (ideal: zero latency, zero hops, zero distance, maximum score). `SelectNearestPeers(n)` returns the n best.

**Persistence**: Writes are debounced with a 5-second coalesce window (`scheduleSave`). The actual write uses an atomic rename pattern (write to `.tmp`, then `os.Rename`) to prevent partial file corruption. `Close()` flushes any pending dirty state synchronously.

**Auth modes**:
- `PeerAuthOpen` — any connecting peer is accepted (default).
- `PeerAuthAllowlist` — only pre-registered peer IDs or explicitly allowlisted public keys are accepted.

**Score bookkeeping**:

| Event | Delta |
|-------|-------|
| Success | +1.0 (capped at 100) |
| Failure | −5.0 (floored at 0) |
| Timeout | −3.0 (floored at 0) |
| Default (new peer) | 50.0 |

**Peer name validation**: Empty names are permitted. Non-empty names must be 1–64 characters, start and end with an alphanumeric character, and contain only alphanumeric, hyphen, underscore, or space characters.

### message.go — Protocol Messages

`Message` is the top-level envelope for all node-to-node communication:

```go
type Message struct {
    ID        string          // UUID v4
    Type      MessageType
    From      string          // Sender node ID
    To        string          // Recipient node ID (empty = broadcast)
    Timestamp time.Time
    Payload   json.RawMessage
    ReplyTo   string          // Set on responses; correlates to original message ID
}
```

**15 message types** across four categories:

| Category | Types |
|----------|-------|
| Connection lifecycle | `handshake`, `handshake_ack`, `ping`, `pong`, `disconnect` |
| Miner operations | `get_stats`, `stats`, `start_miner`, `stop_miner`, `miner_ack` |
| Deployment | `deploy`, `deploy_ack` |
| Logs | `get_logs`, `logs` |
| Error | `error` |

Protocol version negotiation is performed during handshake. `SupportedProtocolVersions` lists all accepted versions (currently `["1.0"]`).

### protocol.go — Response Validation

`ParseResponse` and `ValidateResponse` provide typed helpers for correlating request/response pairs. They check that the response message type matches the expected type and unmarshal the payload into a typed struct.

### worker.go — Command Handlers

`Worker` handles incoming requests on behalf of a node. It processes miner start/stop, stats retrieval, log fetching, and deployment via two decoupled interfaces:

```go
type MinerManager interface {
    StartMiner(config map[string]any) error
    StopMiner(id string) error
    GetStats() map[string]any
    GetLogs(id string, lines int) ([]string, error)
}

type ProfileManager interface {
    ApplyProfile(name string, data []byte) error
}
```

These interfaces allow the worker to be driven by any concrete miner implementation without importing it directly.

### controller.go — Remote Node Operations

`Controller` issues requests to remote peers and correlates responses using a pending-map pattern:

```go
pending map[string]chan *Message  // message ID -> response channel
```

`sendRequest` registers a response channel, sends the message, and blocks with a `context.WithTimeout` until the response arrives or the deadline expires. `handleResponse` (registered as the transport `OnMessage` handler) routes incoming replies to the correct channel by matching `msg.ReplyTo`.

Auto-connect: if the target peer is not yet connected, `sendRequest` calls `transport.Connect` transparently before sending.

`GetAllStats` collects statistics from all connected peers in parallel using goroutines.

### dispatcher.go — UEPS Intent Routing

`Dispatcher` sits between the transport layer and application logic. It routes verified UEPS packets to registered intent handlers after enforcing the threat circuit breaker.

**Threat circuit breaker**: Any packet with `ThreatScore > ThreatScoreThreshold` (50,000) is dropped and logged at WARN level before intent routing begins. The threshold sits at approximately 76% of the `uint16` maximum (50,000 / 65,535), providing headroom for legitimately elevated-risk traffic.

**Intent routing**: Handlers are registered per `IntentID` (1:1 mapping). A `sync.RWMutex` protects the handler map: registration takes a write lock; dispatch takes a read lock (read-heavy workload).

**Well-known intents**:

| Constant | Value | Meaning |
|----------|-------|---------|
| `IntentHandshake` | `0x01` | Connection establishment |
| `IntentCompute` | `0x20` | Compute job request |
| `IntentRehab` | `0x30` | Benevolent intervention (pause execution) |
| `IntentCustom` | `0xFF` | Application-level sub-protocols |

**Sentinel errors**:
- `ErrThreatScoreExceeded` — threat circuit breaker fired
- `ErrUnknownIntent` — no handler registered for the `IntentID`
- `ErrNilPacket` — nil packet passed to `Dispatch`

### bundle.go — TIM Deployment Bundles

`Bundle` wraps an encrypted deployment artefact (profile JSON or miner binary + config). Encryption uses Borg TIM (`tim.ToSigil` / `tim.FromSigil`) with a password-derived key. Integrity is verified with a SHA-256 checksum stored alongside the encrypted data.

Tarball extraction (`extractTarball`) defends against:
- **Zip Slip** — rejects absolute paths and entries containing `..` traversal sequences; verifies every resolved path is still within the destination directory.
- **Decompression bombs** — limits each file to 100 MB.
- **Symlink attacks** — silently skips `tar.TypeSymlink` and `tar.TypeLink` entries.

## ueps/ — UEPS Wire Protocol (RFC-021)

The Unified Encrypted Packet Structure defines a TLV-encoded binary frame authenticated with HMAC-SHA256.

### Packet Format

```
[0x01][len][Version]       Header: Version (0x09 = IPv9)
[0x02][len][CurrentLayer]  Header: Current network layer
[0x03][len][TargetLayer]   Header: Target network layer
[0x04][len][IntentID]      Header: Semantic routing token
[0x05][0x02][ThreatScore]  Header: uint16, big-endian
[0x06][0x20][HMAC-SHA256]  Signature: 32 bytes, covers header TLVs + payload data
[0xFF][...payload...]      Data: no length prefix (relies on external framing)
```

**HMAC coverage**: The signature is computed over the serialised header TLVs (tags 0x01–0x05) concatenated with the raw payload bytes. The HMAC TLV itself (tag 0x06) and the payload tag byte (0xFF) are excluded from the signed data.

### PacketBuilder

`NewBuilder(intentID, payload)` creates a builder with sensible defaults (Version 0x09, layer 5/application, ThreatScore 0). `MarshalAndSign(sharedSecret)` serialises the frame and appends the HMAC.

### ReadAndVerify

`ReadAndVerify(r *bufio.Reader, sharedSecret)` reads a stream, decodes the TLV fields in order, reconstructs the signed data buffer, and verifies the HMAC with `hmac.Equal`. Unknown TLV tags are accumulated into the signed data buffer (forward-compatible extension mechanism) but their semantics are ignored.

**Known limitation**: Tag 0xFF carries no length prefix. The reader calls `io.ReadAll` on the remaining stream, which requires external TCP framing (e.g. a 4-byte length prefix on the enclosing connection) to delimit the packet boundary. The packet is not self-delimiting.

## logging/ — Structured Logger

`Logger` writes structured lines to any `io.Writer` (default: `os.Stderr`) at four levels: DEBUG, INFO, WARN, ERROR. Each line carries a timestamp, level tag, optional component tag, the message string, and key-value pairs.

Format: `2006/01/02 15:04:05 [LEVEL] [component] message | key=value key=value`

A global logger instance is available via `logging.Debug(...)`, `logging.Info(...)`, etc. `logging.New(Config{...})` constructs a scoped logger for use within specific components (e.g., the dispatcher creates one with `Component: "dispatcher"`).

## Concurrency Model

| Resource | Protection |
|----------|------------|
| `Transport.conns` | `sync.RWMutex` |
| `Transport.handler` | `sync.RWMutex` |
| `PeerConnection` writes | `sync.Mutex` (`writeMu`) |
| `PeerConnection` close | `sync.Once` (`closeOnce`) |
| `PeerRegistry.peers` + KD-tree | `sync.RWMutex` |
| `PeerRegistry.allowedPublicKeys` | separate `sync.RWMutex` |
| `PeerRegistry.saveTimer` / `dirty` | `sync.Mutex` (`saveMu`) |
| `Controller.pending` | `sync.RWMutex` |
| `MessageDeduplicator.seen` | `sync.RWMutex` |
| `Dispatcher.handlers` | `sync.RWMutex` |
| `Transport.pendingConns` | `atomic.Int32` |

The codebase is verified race-free under `go test -race`.

## Dependency Graph

```
node/ ──► ueps/
node/ ──► logging/
node/ ──► github.com/Snider/Borg      (STMF crypto, SMSG encryption, TIM)
node/ ──► github.com/Snider/Poindexter (KD-tree peer selection)
node/ ──► github.com/gorilla/websocket
node/ ──► github.com/google/uuid
ueps/ ──► (stdlib only)
logging/ ──► (stdlib only)
```

Borg transitively pulls in Enchantrix (secure environment) and ProtonMail go-crypto.
