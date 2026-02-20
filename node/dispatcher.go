package node

import (
	"fmt"
	"sync"

	"forge.lthn.ai/core/go-p2p/logging"
	"forge.lthn.ai/core/go-p2p/ueps"
)

// ThreatScoreThreshold is the maximum allowable threat score. Packets exceeding
// this value are silently dropped by the circuit breaker and logged as threat
// events. The threshold sits at ~76% of the uint16 range (50,000 / 65,535),
// providing headroom for legitimate elevated-risk traffic whilst rejecting
// clearly hostile payloads.
const ThreatScoreThreshold uint16 = 50000

// Well-known intent identifiers. These correspond to the semantic tokens
// carried in the UEPS IntentID header field (RFC-021).
const (
	IntentHandshake byte = 0x01 // Connection establishment / hello
	IntentCompute   byte = 0x20 // Compute job request
	IntentRehab     byte = 0x30 // Benevolent intervention (pause execution)
	IntentCustom    byte = 0xFF // Extended / application-level sub-protocols
)

// IntentHandler processes a UEPS packet that has been routed by intent.
// Implementations receive the fully parsed and HMAC-verified packet.
type IntentHandler func(pkt *ueps.ParsedPacket) error

// Dispatcher routes verified UEPS packets to registered intent handlers.
// It enforces a threat circuit breaker before routing: any packet whose
// ThreatScore exceeds ThreatScoreThreshold is dropped and logged.
//
// Design decisions:
//   - Handlers are registered per IntentID (1:1 mapping).
//   - Unknown intents are logged at WARN level and silently dropped (no error
//     returned to the caller) to avoid back-pressure on the transport layer.
//   - High-threat packets are dropped silently (logged at WARN) rather than
//     returning an error, consistent with the "don't even parse the payload"
//     philosophy from the original stub.
//   - The dispatcher is safe for concurrent use; a RWMutex protects the
//     handler map.
type Dispatcher struct {
	handlers map[byte]IntentHandler
	mu       sync.RWMutex
	log      *logging.Logger
}

// NewDispatcher creates a Dispatcher with no registered handlers.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[byte]IntentHandler),
		log: logging.New(logging.Config{
			Level:     logging.LevelDebug,
			Component: "dispatcher",
		}),
	}
}

// RegisterHandler associates an IntentHandler with a specific IntentID.
// Calling RegisterHandler with an IntentID that already has a handler will
// replace the previous handler.
func (d *Dispatcher) RegisterHandler(intentID byte, handler IntentHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[intentID] = handler
	d.log.Debug("handler registered", logging.Fields{
		"intent_id": fmt.Sprintf("0x%02X", intentID),
	})
}

// Dispatch routes a parsed UEPS packet through the threat circuit breaker
// and then to the appropriate intent handler.
//
// Behaviour:
//   - Returns ErrThreatScoreExceeded if the packet's ThreatScore exceeds the
//     threshold (packet is dropped and logged).
//   - Returns ErrUnknownIntent if no handler is registered for the IntentID
//     (packet is dropped and logged).
//   - Returns nil on successful delivery to a handler, or any error the
//     handler itself returns.
//   - A nil packet returns ErrNilPacket immediately.
func (d *Dispatcher) Dispatch(pkt *ueps.ParsedPacket) error {
	if pkt == nil {
		return ErrNilPacket
	}

	// 1. Threat circuit breaker (L5 guard)
	if pkt.Header.ThreatScore > ThreatScoreThreshold {
		d.log.Warn("packet dropped: threat score exceeds safety threshold", logging.Fields{
			"threat_score": pkt.Header.ThreatScore,
			"threshold":    ThreatScoreThreshold,
			"intent_id":    fmt.Sprintf("0x%02X", pkt.Header.IntentID),
			"version":      pkt.Header.Version,
		})
		return ErrThreatScoreExceeded
	}

	// 2. Intent routing (L9 semantic)
	d.mu.RLock()
	handler, exists := d.handlers[pkt.Header.IntentID]
	d.mu.RUnlock()

	if !exists {
		d.log.Warn("packet dropped: unknown intent", logging.Fields{
			"intent_id": fmt.Sprintf("0x%02X", pkt.Header.IntentID),
			"version":   pkt.Header.Version,
		})
		return ErrUnknownIntent
	}

	return handler(pkt)
}

// Sentinel errors returned by Dispatch.
var (
	// ErrThreatScoreExceeded is returned when a packet's ThreatScore exceeds
	// the safety threshold.
	ErrThreatScoreExceeded = fmt.Errorf("packet rejected: threat score exceeds safety threshold (%d)", ThreatScoreThreshold)

	// ErrUnknownIntent is returned when no handler is registered for the
	// packet's IntentID.
	ErrUnknownIntent = fmt.Errorf("packet dropped: unknown intent")

	// ErrNilPacket is returned when a nil packet is passed to Dispatch.
	ErrNilPacket = fmt.Errorf("dispatch: nil packet")
)
