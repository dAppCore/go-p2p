---
title: TIM Deployment Bundles
description: Encrypted deployment bundles using TIM/STIM format with Zip Slip defences.
---

# TIM Deployment Bundles

The bundle system handles encrypted deployment packages for peer-to-peer transfer. Bundles use the TIM (Terminal Isolation Matrix) format from the Borg library for encryption and the tar format for file archives. Extraction includes multiple layers of path traversal defence.

**File:** `node/bundle.go`

## Bundle Types

```go
const (
    BundleProfile BundleType = "profile" // JSON configuration only
    BundleMiner   BundleType = "miner"   // Miner binary + optional config
    BundleFull    BundleType = "full"    // Everything (miner + profiles + config)
)
```

## Bundle Structure

```go
type Bundle struct {
    Type     BundleType `json:"type"`
    Name     string     `json:"name"`
    Data     []byte     `json:"data"`     // Encrypted STIM data or raw JSON
    Checksum string     `json:"checksum"` // SHA-256 of Data
}
```

Every bundle carries a SHA-256 checksum of its `Data` field. This is verified before extraction to detect corruption or tampering in transit.

## Creating Bundles

### Profile Bundle (Encrypted)

```go
profileJSON := []byte(`{"algorithm":"randomx","threads":4}`)
bundle, err := node.CreateProfileBundle(profileJSON, "gpu-profile", password)
```

Internally:
1. Creates a TIM container with the profile as its config.
2. Encrypts to STIM format using the password.
3. Computes SHA-256 checksum.

### Profile Bundle (Unencrypted)

For testing or trusted networks:

```go
bundle, err := node.CreateProfileBundleUnencrypted(profileJSON, "test-profile")
```

### Miner Bundle

```go
bundle, err := node.CreateMinerBundle(
    "/path/to/xmrig",     // Miner binary path
    profileJSON,           // Optional profile config (may be nil)
    "xmrig-linux-amd64",  // Bundle name
    password,              // Encryption password
)
```

Internally:
1. Reads the miner binary from disk.
2. Creates a tar archive containing the binary.
3. Converts the tar to a Borg DataNode, then to a TIM container.
4. Attaches the profile config if provided.
5. Encrypts to STIM format.

## Extracting Bundles

### Profile Bundle

```go
profileJSON, err := node.ExtractProfileBundle(bundle, password)
```

Detects whether the data is plain JSON or encrypted STIM and handles both cases.

### Miner Bundle

```go
minerPath, profileJSON, err := node.ExtractMinerBundle(bundle, password, "/opt/miners/")
```

Returns the path to the first executable found in the archive and the profile config.

## Verification

```go
ok := node.VerifyBundle(bundle)
```

Recomputes the SHA-256 checksum and compares against the stored value.

## Streaming

For large bundles transferred over the network:

```go
// Write
err := node.StreamBundle(bundle, writer)

// Read
bundle, err := node.ReadBundle(reader)
```

Both use JSON encoding/decoding via `json.Encoder`/`json.Decoder`.

## Security: Zip Slip Defence

Tar extraction (`extractTarball`) applies multiple layers of path traversal protection:

1. **Path cleaning** -- `filepath.Clean(hdr.Name)` normalises the entry name.
2. **Absolute path rejection** -- entries with absolute paths are rejected.
3. **Parent directory traversal** -- entries starting with `../` or equal to `..` are rejected.
4. **Destination containment** -- the resolved full path must have `destDir` as a prefix. This catches edge cases the previous checks might miss.
5. **Symlink rejection** -- both symbolic links (`TypeSymlink`) and hard links (`TypeLink`) are silently skipped, preventing symlink-based escapes.
6. **File size limit** -- each file is capped at 100MB via `io.LimitReader` to prevent decompression bombs. Files exceeding the limit are deleted after detection.

### Example of a Rejected Entry

```
Entry: "../../../etc/passwd"
  -> filepath.Clean -> "../../../etc/passwd"
  -> strings.HasPrefix(".." + "/") -> true
  -> REJECTED: "invalid tar entry: path traversal attempt"
```

## Deployment Message Flow

The bundle system integrates with the P2P message protocol:

```
Controller                              Worker
    |                                     |
    |  [CreateProfileBundle / CreateMinerBundle]
    |                                     |
    |--- deploy (DeployPayload) --------->|
    |                                     |
    |   [VerifyBundle]                    |
    |   [ExtractProfileBundle / ExtractMinerBundle]
    |                                     |
    |<-- deploy_ack (success/error) ------|
```

### DeployPayload

```go
type DeployPayload struct {
    BundleType string `json:"type"`     // "profile", "miner", or "full"
    Data       []byte `json:"data"`     // STIM-encrypted bundle
    Checksum   string `json:"checksum"` // SHA-256 of Data
    Name       string `json:"name"`     // Profile or miner name
}
```

## BundleManifest

Describes the contents of a bundle for catalogue purposes:

```go
type BundleManifest struct {
    Type       BundleType `json:"type"`
    Name       string     `json:"name"`
    Version    string     `json:"version,omitempty"`
    MinerType  string     `json:"minerType,omitempty"`
    ProfileIDs []string   `json:"profileIds,omitempty"`
    CreatedAt  string     `json:"createdAt"`
}
```

## Dependencies

| Library | Usage |
|---------|-------|
| `forge.lthn.ai/Snider/Borg/pkg/tim` | TIM container creation, STIM encryption/decryption |
| `forge.lthn.ai/Snider/Borg/pkg/datanode` | DataNode from tar archive (for miner bundles) |
| `archive/tar` | Tar creation and extraction |
| `crypto/sha256` | Bundle checksum computation |
