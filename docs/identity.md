---
title: Node Identity
description: X25519 keypair generation, node ID derivation, and HMAC-SHA256 challenge-response authentication.
---

# Node Identity

Every node in the mesh has a unique identity derived from an X25519 keypair. The node ID is cryptographically bound to the public key, and authentication uses HMAC-SHA256 challenge-response over a shared secret derived via ECDH.

## NodeIdentity

The public identity struct carried in handshake messages and stored on disk:

```go
type NodeIdentity struct {
    ID        string    `json:"id"`        // 32-char hex, derived from public key
    Name      string    `json:"name"`      // Human-friendly name
    PublicKey string    `json:"publicKey"` // X25519 base64
    CreatedAt time.Time `json:"createdAt"`
    Role      NodeRole  `json:"role"`      // controller, worker, or dual
}
```

The `ID` is computed as the first 16 bytes of `SHA-256(publicKey)`, hex-encoded to produce a 32-character string.

## Key Storage

| Item | Path | Permissions |
|------|------|-------------|
| Private key | `~/.local/share/lethean-desktop/node/private.key` | `0600` |
| Identity config | `~/.config/lethean-desktop/node.json` | `0644` |

Paths follow XDG base directories via `github.com/adrg/xdg`. The private key is never serialised to JSON or transmitted over the network.

## NodeManager

`NodeManager` handles identity lifecycle -- generation, persistence, loading, and deletion. It also derives shared secrets for peer authentication.

### Creating an Identity

```go
nm, err := node.NewNodeManager()
if err != nil {
    log.Fatal(err)
}

// Generate a new identity (persists key and config to disk)
err = nm.GenerateIdentity("eu-controller-01", node.RoleController)
```

Internally this calls `stmf.GenerateKeyPair()` from the Borg library to produce the X25519 keypair.

### Custom Paths (Testing)

```go
nm, err := node.NewNodeManagerWithPaths(
    "/tmp/test/private.key",
    "/tmp/test/node.json",
)
```

### Checking and Retrieving Identity

```go
if nm.HasIdentity() {
    identity := nm.GetIdentity() // Returns a copy
    fmt.Println(identity.ID, identity.Name)
}
```

`GetIdentity()` returns a copy of the identity struct to prevent mutation of the internal state.

### Deriving Shared Secrets

```go
sharedSecret, err := nm.DeriveSharedSecret(peerPublicKeyBase64)
```

This performs X25519 ECDH with the peer's public key and hashes the result with SHA-256, producing a 32-byte symmetric key. The same shared secret is derived independently by both sides (no secret is transmitted).

### Deleting an Identity

```go
err := nm.Delete() // Removes key and config from disk, clears in-memory state
```

## Challenge-Response Authentication

After the ECDH key exchange, nodes prove identity possession through HMAC-SHA256 challenge-response. The shared secret is never transmitted.

### Functions

```go
// Generate a 32-byte cryptographically random challenge
challenge, err := node.GenerateChallenge()

// Sign a challenge with the shared secret (HMAC-SHA256)
response := node.SignChallenge(challenge, sharedSecret)

// Verify a challenge response (constant-time comparison via hmac.Equal)
ok := node.VerifyChallenge(challenge, response, sharedSecret)
```

### Authentication Flow

```
Node A (initiator)                    Node B (responder)
  |                                      |
  |--- handshake (identity + challenge) ->|
  |                                      |
  |   [B derives shared secret via ECDH]  |
  |   [B checks protocol version]        |
  |   [B checks allowlist]               |
  |   [B signs challenge with HMAC]       |
  |                                      |
  |<-- handshake_ack (identity + sig) ---|
  |                                      |
  |   [A derives shared secret via ECDH]  |
  |   [A verifies challenge response]     |
  |                                      |
  |   ---- encrypted channel open ----    |
```

The handshake and handshake_ack messages are sent unencrypted (they carry the public keys needed to derive the shared secret). All subsequent messages are SMSG-encrypted.

## Node Roles

```go
const (
    RoleController NodeRole = "controller"  // Manages the mesh, distributes tasks
    RoleWorker     NodeRole = "worker"      // Receives and executes workloads
    RoleDual       NodeRole = "dual"        // Both controller and worker
)
```

| Role | Behaviour |
|------|-----------|
| `controller` | Sends `get_stats`, `start_miner`, `stop_miner`, `get_logs`, `deploy` messages |
| `worker` | Handles incoming commands, runs compute tasks, reports stats |
| `dual` | Participates as both controller and worker |

## Thread Safety

`NodeManager` is safe for concurrent use. A `sync.RWMutex` protects all internal state. `GetIdentity()` returns a copy rather than a pointer to the internal struct.
