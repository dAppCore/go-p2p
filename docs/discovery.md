---
title: Peer Discovery
description: KD-tree peer selection across four weighted dimensions, score tracking, and allowlist authentication.
---

# Peer Discovery

The `PeerRegistry` manages known peers with intelligent selection via a 4-dimensional KD-tree (powered by Poindexter). Peers are scored based on network metrics and reliability, persisted with debounced writes, and gated by configurable authentication modes.

## Peer Structure

```go
type Peer struct {
    ID        string    `json:"id"`        // Node ID (derived from public key)
    Name      string    `json:"name"`      // Human-readable name
    PublicKey string    `json:"publicKey"` // X25519 public key (base64)
    Address   string    `json:"address"`   // host:port for WebSocket connection
    Role      NodeRole  `json:"role"`      // controller, worker, or dual
    AddedAt   time.Time `json:"addedAt"`
    LastSeen  time.Time `json:"lastSeen"`

    // Poindexter metrics (updated dynamically)
    PingMS float64 `json:"pingMs"` // Latency in milliseconds
    Hops   int     `json:"hops"`   // Network hop count
    GeoKM  float64 `json:"geoKm"`  // Geographic distance in kilometres
    Score  float64 `json:"score"`  // Reliability score 0--100

    Connected bool `json:"-"` // Not persisted
}
```

## Authentication Modes

```go
const (
    PeerAuthOpen      PeerAuthMode = iota // Accept any peer that authenticates
    PeerAuthAllowlist                      // Only accept pre-approved peers
)
```

| Mode | Behaviour |
|------|-----------|
| `PeerAuthOpen` | Any node that completes the challenge-response handshake is accepted |
| `PeerAuthAllowlist` | Only nodes whose public keys appear on the allowlist, or that are already registered, are accepted |

### Allowlist Management

```go
registry.SetAuthMode(node.PeerAuthAllowlist)

// Add/revoke public keys
registry.AllowPublicKey(peerPublicKeyBase64)
registry.RevokePublicKey(peerPublicKeyBase64)

// Check
ok := registry.IsPublicKeyAllowed(peerPublicKeyBase64)

// List all allowed keys
keys := registry.ListAllowedPublicKeys()

// Iterate
for key := range registry.AllowedPublicKeys() {
    // ...
}
```

The `IsPeerAllowed(peerID, publicKey)` method is called during the transport handshake. It returns `true` if:
- Auth mode is `PeerAuthOpen`, **or**
- The peer ID is already registered in the registry, **or**
- The public key is on the allowlist.

### Peer Name Validation

Peer names are validated on `AddPeer()`:
- 1--64 characters
- Must start and end with alphanumeric characters
- May contain alphanumeric, hyphens, underscores, and spaces
- Empty names are permitted (the field is optional)

## KD-Tree Peer Selection

The registry maintains a 4-dimensional KD-tree for optimal peer selection. Each peer is represented as a weighted point:

| Dimension | Source | Weight | Direction |
|-----------|--------|--------|-----------|
| Latency | `PingMS` | 1.0 | Lower is better |
| Hops | `Hops` | 0.7 | Lower is better |
| Geographic distance | `GeoKM` | 0.2 | Lower is better |
| Reliability | `100 - Score` | 1.2 | Inverted so lower is better |

The score dimension is inverted so that the "ideal peer" target point `[0, 0, 0, 0]` represents zero latency, zero hops, zero distance, and maximum reliability (score 100).

### Selecting Peers

```go
// Best single peer (nearest to ideal in 4D space)
best := registry.SelectOptimalPeer()

// Top N peers
top3 := registry.SelectNearestPeers(3)

// All peers sorted by score (highest first)
ranked := registry.GetPeersByScore()

// Iterator over peers by score
for peer := range registry.PeersByScore() {
    // ...
}
```

The KD-tree uses Euclidean distance (configured via `poindexter.WithMetric(poindexter.EuclideanDistance{})`). It is rebuilt whenever peers are added, removed, or their metrics change.

## Score Tracking

Peer reliability is tracked with a score between 0 and 100 (default 50 for new peers).

```go
const (
    ScoreSuccessIncrement = 1.0   // +1 per successful interaction
    ScoreFailureDecrement = 5.0   // -5 per failure
    ScoreTimeoutDecrement = 3.0   // -3 per timeout
)
```

```go
registry.RecordSuccess(peerID)  // Score += 1, capped at 100
registry.RecordFailure(peerID)  // Score -= 5, floored at 0
registry.RecordTimeout(peerID)  // Score -= 3, floored at 0

// Direct score update (clamped to 0--100)
registry.UpdateScore(peerID, 75.0)
```

The asymmetric adjustments (+1 success vs -5 failure) mean that a peer must sustain consistent reliability to maintain a high score. A single failure costs five successful interactions to recover.

## Metric Updates

```go
registry.UpdateMetrics(peerID, pingMS, geoKM, hops)
```

This also updates `LastSeen` and triggers a KD-tree rebuild.

## Registry Operations

```go
// Create
registry, err := node.NewPeerRegistry()             // XDG paths
registry, err := node.NewPeerRegistryWithPath(path)  // Custom path (testing)

// CRUD
err := registry.AddPeer(peer)
err := registry.UpdatePeer(peer)
err := registry.RemovePeer(peerID)
peer := registry.GetPeer(peerID)    // Returns a copy

// Lists and iterators
peers := registry.ListPeers()
count := registry.Count()

for peer := range registry.Peers() {
    // Each peer is a copy to prevent mutation
}

// Connection state
registry.SetConnected(peerID, true)
connected := registry.GetConnectedPeers()

for peer := range registry.ConnectedPeers() {
    // ...
}
```

## Persistence

Peers are persisted to `~/.config/lethean-desktop/peers.json` as a JSON array.

### Debounced Writes

To avoid excessive disk I/O, saves are debounced with a 5-second coalesce interval. Multiple mutations within that window produce a single disk write. The write uses an atomic rename pattern (write to `.tmp`, then `os.Rename`) to prevent corruption on crash.

```go
// Flush pending changes on shutdown
err := registry.Close()
```

Always call `Close()` before shutdown to ensure unsaved changes are flushed.

## Peer Lifecycle

```
Discovery          Authentication         Active              Stale
   |                    |                   |                   |
   |  handshake    +---------+     +--------+------+     +-----+
   +-------------->| Auth    |---->| Score  | Ping |---->| Evict|
                   | Check   |     | Update | Loop |     | or   |
                   +---------+     +--------+------+     | Retry|
                        |                                 +-----+
                   [rejected]
```

1. **Discovery** -- New peer connects or is discovered via mesh communication.
2. **Authentication** -- Challenge-response handshake (see [identity.md](identity.md)). Allowlist checked if in allowlist mode.
3. **Active** -- Metrics updated via ping, score adjusted on success/failure, eligible for task assignment.
4. **Stale** -- No response after keepalive timeout; connection removed, `Connected` set to `false`.

## Thread Safety

`PeerRegistry` is safe for concurrent use. A `sync.RWMutex` protects the peer map and KD-tree. Allowlist operations use a separate `sync.RWMutex`. All public methods that return peers return copies to prevent external mutation.
