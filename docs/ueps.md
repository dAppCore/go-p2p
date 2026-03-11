---
title: UEPS Wire Protocol
description: TLV-encoded wire protocol with HMAC-SHA256 integrity verification (RFC-021).
---

# UEPS Wire Protocol

The `ueps` package implements the Universal Encrypted Payload System -- a consent-gated TLV (Type-Length-Value) wire protocol with HMAC-SHA256 integrity verification. This is the low-level binary protocol that sits beneath the JSON-over-WebSocket mesh layer.

**Package:** `forge.lthn.ai/core/go-p2p/ueps`

## TLV Format

Each field is encoded as a 1-byte tag, 2-byte big-endian length (uint16), and variable-length value. Maximum field size is 65,535 bytes.

```
+------+--------+--------+-----------+
| Tag  | Len-Hi | Len-Lo |   Value   |
| 1B   | 1B     | 1B     | 0..65535B |
+------+--------+--------+-----------+
```

## Tag Registry

| Tag | Constant | Value Size | Description |
|-----|----------|------------|-------------|
| `0x01` | `TagVersion` | 1 byte | Protocol version (default `0x09` for IPv9) |
| `0x02` | `TagCurrentLay` | 1 byte | Current network layer |
| `0x03` | `TagTargetLay` | 1 byte | Target network layer |
| `0x04` | `TagIntent` | 1 byte | Semantic intent token (routes the packet) |
| `0x05` | `TagThreatScore` | 2 bytes | Threat score (0--65535, big-endian uint16) |
| `0x06` | `TagHMAC` | 32 bytes | HMAC-SHA256 signature |
| `0xFF` | `TagPayload` | variable | Application data |

## Header

```go
type UEPSHeader struct {
    Version      uint8   // Default 0x09
    CurrentLayer uint8   // Source layer
    TargetLayer  uint8   // Destination layer
    IntentID     uint8   // Semantic intent token
    ThreatScore  uint16  // 0--65535
}
```

## Building Packets

`PacketBuilder` constructs signed UEPS frames:

```go
builder := ueps.NewBuilder(intentID, payload)
builder.Header.ThreatScore = 100

frame, err := builder.MarshalAndSign(sharedSecret)
```

### Defaults

`NewBuilder` sets:
- `Version`: `0x09` (IPv9)
- `CurrentLayer`: `5` (Application)
- `TargetLayer`: `5` (Application)
- `ThreatScore`: `0` (assumed innocent)

### Wire Layout

`MarshalAndSign` produces:

```
[Version TLV][CurrentLayer TLV][TargetLayer TLV][Intent TLV][ThreatScore TLV][HMAC TLV][Payload TLV]
```

1. Serialises header TLVs (tags `0x01`--`0x05`) into the buffer.
2. Computes HMAC-SHA256 over `header_bytes + raw_payload` using the shared secret.
3. Writes the HMAC TLV (`0x06`, 32 bytes).
4. Writes the payload TLV (`0xFF`, with 2-byte length prefix).

### What the HMAC Covers

The signature covers:

- All header TLV bytes (tag + length + value for tags `0x01`--`0x05`)
- The raw payload bytes

It does **not** cover the HMAC TLV itself or the payload's tag/length bytes (only the payload value).

## Reading and Verifying

`ReadAndVerify` parses and validates a UEPS frame from a buffered reader:

```go
packet, err := ueps.ReadAndVerify(bufio.NewReader(data), sharedSecret)
if err != nil {
    // Integrity violation or malformed packet
}

fmt.Println(packet.Header.IntentID)
fmt.Println(string(packet.Payload))
```

### Verification Steps

1. Reads TLV fields sequentially, accumulating header bytes into a signed-data buffer.
2. Stores the HMAC signature separately (not added to signed-data).
3. On encountering `0xFF` (payload tag), reads the length-prefixed payload.
4. Recomputes HMAC over `signed_data + payload`.
5. Compares signatures using `hmac.Equal` (constant-time).

On HMAC mismatch, returns: `"integrity violation: HMAC mismatch (ThreatScore +100)"`.

### Parsed Result

```go
type ParsedPacket struct {
    Header  UEPSHeader
    Payload []byte
}
```

### Unknown Tags

Unknown tags between the header and HMAC are included in the signed-data buffer but ignored semantically. This provides forward compatibility -- older readers can verify packets that include newer header fields.

## Roundtrip Example

```go
secret := []byte("shared-secret-32-bytes-here.....")
payload := []byte(`{"action":"compute","params":{}}`)

// Build and sign
builder := ueps.NewBuilder(0x20, payload) // IntentCompute
frame, err := builder.MarshalAndSign(secret)
if err != nil {
    log.Fatal(err)
}

// Read and verify
reader := bufio.NewReader(bytes.NewReader(frame))
packet, err := ueps.ReadAndVerify(reader, secret)
if err != nil {
    log.Fatal(err) // Integrity violation
}

fmt.Printf("Intent: 0x%02X\n", packet.Header.IntentID)   // 0x20
fmt.Printf("Payload: %s\n", string(packet.Payload))
```

## Intent Routing

The `IntentID` field enables semantic routing at the application layer. The [dispatcher](routing.md) uses this field to route verified packets to registered handlers.

Reserved intent values:

| ID | Constant | Purpose |
|----|----------|---------|
| `0x01` | `IntentHandshake` | Connection establishment / hello |
| `0x20` | `IntentCompute` | Compute job request |
| `0x30` | `IntentRehab` | Benevolent intervention (pause execution) |
| `0xFF` | `IntentCustom` | Extended / application-level sub-protocols |

## Threat Score

The `ThreatScore` field (0--65535) provides a mechanism for nodes to signal the perceived risk level of a packet. The dispatcher's circuit breaker drops packets exceeding a threshold of 50,000 (see [routing.md](routing.md)).

When the reader detects an HMAC mismatch, the error message includes `ThreatScore +100` as guidance for upstream threat tracking.

## Security Notes

- HMAC-SHA256 provides both integrity and authenticity (assuming the shared secret is known only to the two communicating nodes).
- The constant-time comparison via `hmac.Equal` prevents timing side-channel attacks.
- The 2-byte length prefix on all TLVs (including payload) prevents unbounded reads -- maximum 65,535 bytes per field.
