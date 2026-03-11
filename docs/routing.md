---
title: Intent Routing
description: UEPS intent-based packet routing with threat circuit breaker.
---

# Intent Routing

The `Dispatcher` routes verified UEPS packets to registered intent handlers. Before routing, it enforces a threat circuit breaker that silently drops packets with elevated threat scores.

**File:** `node/dispatcher.go`

## Dispatcher

```go
dispatcher := node.NewDispatcher()
```

The dispatcher is safe for concurrent use -- a `sync.RWMutex` protects the handler map.

## Registering Handlers

Handlers are registered per intent ID (one handler per intent). Registering a new handler for an existing intent replaces the previous one.

```go
dispatcher.RegisterHandler(node.IntentHandshake, func(pkt *ueps.ParsedPacket) error {
    // Handle handshake packets
    return nil
})

dispatcher.RegisterHandler(node.IntentCompute, func(pkt *ueps.ParsedPacket) error {
    // Handle compute job requests
    return nil
})
```

### Handler Signature

```go
type IntentHandler func(pkt *ueps.ParsedPacket) error
```

Handlers receive the fully parsed and HMAC-verified packet. The payload bytes are available as `pkt.Payload` and the routing metadata as `pkt.Header`.

### Iterating Handlers

```go
for intentID, handler := range dispatcher.Handlers() {
    fmt.Printf("0x%02X registered\n", intentID)
}
```

## Dispatching Packets

```go
err := dispatcher.Dispatch(packet)
```

### Dispatch Flow

1. **Nil check** -- returns `ErrNilPacket` immediately.
2. **Threat circuit breaker** -- if `ThreatScore > 50,000`, the packet is dropped and `ErrThreatScoreExceeded` is returned. A warning is logged.
3. **Intent lookup** -- finds the handler registered for `pkt.Header.IntentID`. If none exists, the packet is dropped and `ErrUnknownIntent` is returned.
4. **Handler invocation** -- calls the handler and returns its result.

### Threat Circuit Breaker

```go
const ThreatScoreThreshold uint16 = 50000
```

The threshold sits at approximately 76% of the uint16 range (50,000 / 65,535). This provides headroom for legitimate elevated-risk traffic whilst rejecting clearly hostile payloads.

Dropped packets are logged at WARN level with the threat score, threshold, intent ID, and protocol version.

### Design Rationale

- **High-threat packets are dropped silently** (from the sender's perspective) rather than returning an error, consistent with the "don't even parse the payload" philosophy.
- **Unknown intents are dropped**, not forwarded, to avoid back-pressure on the transport layer. They are logged at WARN level for debugging.
- **Handler errors propagate** to the caller, allowing upstream code to record failures.

## Intent Constants

```go
const (
    IntentHandshake byte = 0x01 // Connection establishment / hello
    IntentCompute   byte = 0x20 // Compute job request
    IntentRehab     byte = 0x30 // Benevolent intervention (pause execution)
    IntentCustom    byte = 0xFF // Extended / application-level sub-protocols
)
```

| Intent | ID | Purpose |
|--------|----|---------|
| Handshake | `0x01` | Connection establishment and hello exchange |
| Compute | `0x20` | Compute job requests (mining, inference, etc.) |
| Rehab | `0x30` | Benevolent intervention -- pause execution of a potentially harmful workload |
| Custom | `0xFF` | Application-level sub-protocols carried in the payload |

## Sentinel Errors

```go
var (
    ErrThreatScoreExceeded = fmt.Errorf(
        "packet rejected: threat score exceeds safety threshold (%d)",
        ThreatScoreThreshold,
    )
    ErrUnknownIntent = errors.New("packet dropped: unknown intent")
    ErrNilPacket     = errors.New("dispatch: nil packet")
)
```

## Integration with Transport

The dispatcher operates above the UEPS reader. A typical integration:

```go
// Parse and verify the UEPS frame
packet, err := ueps.ReadAndVerify(reader, sharedSecret)
if err != nil {
    // HMAC mismatch or malformed packet
    return err
}

// Route through the dispatcher
if err := dispatcher.Dispatch(packet); err != nil {
    if errors.Is(err, node.ErrThreatScoreExceeded) {
        // Packet was too threatening -- already logged
        return nil
    }
    if errors.Is(err, node.ErrUnknownIntent) {
        // No handler for this intent -- already logged
        return nil
    }
    // Handler returned an error
    return err
}
```

## Full Example

```go
// Set up dispatcher with handlers
dispatcher := node.NewDispatcher()

dispatcher.RegisterHandler(node.IntentCompute, func(pkt *ueps.ParsedPacket) error {
    fmt.Printf("Compute request: %s\n", string(pkt.Payload))
    return nil
})

// Build a packet
builder := ueps.NewBuilder(node.IntentCompute, []byte(`{"job":"hashrate"}`))
frame, _ := builder.MarshalAndSign(sharedSecret)

// Parse and dispatch
packet, _ := ueps.ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), sharedSecret)
err := dispatcher.Dispatch(packet) // Calls the compute handler
```
