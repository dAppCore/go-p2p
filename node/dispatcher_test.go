package node

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"dappco.re/go/core/p2p/ueps"
)

// makePacket builds a minimal ParsedPacket for testing. ThreatScore defaults
// to 0 (safe) and Version to 0x09 (current protocol).
func makePacket(intentID byte, threatScore uint16, payload []byte) *ueps.ParsedPacket {
	return &ueps.ParsedPacket{
		Header: ueps.UEPSHeader{
			Version:      0x09,
			CurrentLayer: 5,
			TargetLayer:  5,
			IntentID:     intentID,
			ThreatScore:  threatScore,
		},
		Payload: payload,
	}
}

// --- Dispatcher Tests ---

func TestDispatcher_RegisterAndDispatch(t *testing.T) {
	t.Run("handler receives the correct packet", func(t *testing.T) {
		d := NewDispatcher()
		var received *ueps.ParsedPacket

		d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
			received = pkt
			return nil
		})

		pkt := makePacket(IntentHandshake, 0, []byte("hello"))
		err := d.Dispatch(pkt)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if received == nil {
			t.Fatal("expected non-nil")
		}
		if !reflect.DeepEqual(pkt, received) {
			t.Fatalf("want %v, got %v", pkt, received)
		}
		if !reflect.DeepEqual([]byte("hello"), received.Payload) {
			t.Fatalf("want %v, got %v", []byte("hello"), received.Payload)
		}
	})

	t.Run("handler error propagates to caller", func(t *testing.T) {
		d := NewDispatcher()
		handlerErr := fmt.Errorf("compute failed")

		d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
			return handlerErr
		})

		pkt := makePacket(IntentCompute, 0, []byte("job"))
		err := d.Dispatch(pkt)

		if !errors.Is(err, handlerErr) {
			t.Fatalf("expected error %v, got %v", handlerErr, err)
		}
	})
}

func TestDispatcher_ThreatCircuitBreaker(t *testing.T) {
	tests := []struct {
		name        string
		threatScore uint16
		wantErr     error
		dispatched  bool
	}{
		{
			name:        "score at threshold is allowed",
			threatScore: ThreatScoreThreshold,
			wantErr:     nil,
			dispatched:  true,
		},
		{
			name:        "score just above threshold is rejected",
			threatScore: ThreatScoreThreshold + 1,
			wantErr:     ErrThreatScoreExceeded,
			dispatched:  false,
		},
		{
			name:        "maximum uint16 score is rejected",
			threatScore: 65535,
			wantErr:     ErrThreatScoreExceeded,
			dispatched:  false,
		},
		{
			name:        "zero score is allowed",
			threatScore: 0,
			wantErr:     nil,
			dispatched:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher()
			var called bool

			d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
				called = true
				return nil
			})

			pkt := makePacket(IntentHandshake, tt.threatScore, []byte("data"))
			err := d.Dispatch(pkt)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if !reflect.DeepEqual(tt.dispatched, called) {
				t.Fatalf("want %v, got %v", tt.dispatched, called)
			}
		})
	}
}

func TestDispatcher_UnknownIntentDropped(t *testing.T) {
	d := NewDispatcher()

	// Register handlers for known intents only
	d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
		return nil
	})

	// Dispatch a packet with an unregistered intent (0x42)
	pkt := makePacket(0x42, 0, []byte("unknown"))
	err := d.Dispatch(pkt)

	if !errors.Is(err, ErrUnknownIntent) {
		t.Fatalf("expected error %v, got %v", ErrUnknownIntent, err)
	}
}

func TestDispatcher_MultipleHandlersCorrectRouting(t *testing.T) {
	d := NewDispatcher()

	var handshakeCalled, computeCalled, rehabCalled, customCalled bool

	d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
		handshakeCalled = true
		return nil
	})
	d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		computeCalled = true
		return nil
	})
	d.RegisterHandler(IntentRehab, func(pkt *ueps.ParsedPacket) error {
		rehabCalled = true
		return nil
	})
	d.RegisterHandler(IntentCustom, func(pkt *ueps.ParsedPacket) error {
		customCalled = true
		return nil
	})

	tests := []struct {
		name     string
		intentID byte
		want     *bool
	}{
		{"handshake routes correctly", IntentHandshake, &handshakeCalled},
		{"compute routes correctly", IntentCompute, &computeCalled},
		{"rehab routes correctly", IntentRehab, &rehabCalled},
		{"custom routes correctly", IntentCustom, &customCalled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset all flags
			handshakeCalled = false
			computeCalled = false
			rehabCalled = false
			customCalled = false

			pkt := makePacket(tt.intentID, 0, []byte("payload"))
			err := d.Dispatch(pkt)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !(*tt.want) {
				t.Fatal("expected true")
			}

			// Verify no other handler was called
			for _, other := range tests {
				if other.intentID != tt.intentID {
					if *other.want {
						t.Fatal("expected false")
					}
				}
			}
		})
	}
}

func TestDispatcher_NilAndEmptyPayload(t *testing.T) {
	t.Run("nil packet returns ErrNilPacket", func(t *testing.T) {
		d := NewDispatcher()
		err := d.Dispatch(nil)
		if !errors.Is(err, ErrNilPacket) {
			t.Fatalf("expected error %v, got %v", ErrNilPacket, err)
		}
	})

	t.Run("nil payload is delivered to handler", func(t *testing.T) {
		d := NewDispatcher()
		var received *ueps.ParsedPacket

		d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
			received = pkt
			return nil
		})

		pkt := makePacket(IntentHandshake, 0, nil)
		err := d.Dispatch(pkt)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if received == nil {
			t.Fatal("expected non-nil")
		}
		if received.Payload != nil {
			t.Fatalf("expected nil, got %v", received.Payload)
		}
	})

	t.Run("empty payload is delivered to handler", func(t *testing.T) {
		d := NewDispatcher()
		var received *ueps.ParsedPacket

		d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
			received = pkt
			return nil
		})

		pkt := makePacket(IntentHandshake, 0, []byte{})
		err := d.Dispatch(pkt)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if received == nil {
			t.Fatal("expected non-nil")
		}
		if len(received.Payload) != 0 {
			t.Fatalf("expected empty, got %v", received.Payload)
		}
	})
}

func TestDispatcher_ConcurrentDispatchSafety(t *testing.T) {
	d := NewDispatcher()

	var count atomic.Int64

	d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		count.Add(1)
		return nil
	})

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			pkt := makePacket(IntentCompute, 0, []byte("concurrent"))
			err := d.Dispatch(pkt)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()
	if !reflect.DeepEqual(int64(goroutines), count.Load()) {
		t.Fatalf("want %v, got %v", int64(goroutines), count.Load())
	}
}

func TestDispatcher_ConcurrentRegisterAndDispatch(t *testing.T) {
	d := NewDispatcher()

	var count atomic.Int64

	// Pre-register a handler so dispatches have something to hit
	d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
		count.Add(1)
		return nil
	})

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines dispatch packets
	for range goroutines {
		go func() {
			defer wg.Done()
			pkt := makePacket(IntentHandshake, 0, []byte("data"))
			_ = d.Dispatch(pkt)
		}()
	}

	// Half the goroutines register/replace handlers concurrently
	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			d.RegisterHandler(byte(n%4), func(pkt *ueps.ParsedPacket) error {
				return nil
			})
		}(i)
	}

	wg.Wait()
	// We only assert no panics / races occurred; count may vary depending
	// on scheduling order.
	if !(count.Load() >= 0) {
		t.Fatal("expected true")
	}
}

func TestDispatcher_ReplaceHandler(t *testing.T) {
	d := NewDispatcher()

	var firstCalled, secondCalled bool

	d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		firstCalled = true
		return nil
	})

	// Replace the handler
	d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		secondCalled = true
		return nil
	})

	pkt := makePacket(IntentCompute, 0, []byte("replaced"))
	err := d.Dispatch(pkt)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if firstCalled {
		t.Fatal("expected false")
	}
	if !(secondCalled) {
		t.Fatal("expected true")
	}
}

func TestDispatcher_ThreatBlocksBeforeRouting(t *testing.T) {
	// Verify that the circuit breaker fires before intent routing,
	// so even an unknown intent returns ErrThreatScoreExceeded (not ErrUnknownIntent).
	d := NewDispatcher()

	pkt := makePacket(0x42, ThreatScoreThreshold+1, []byte("hostile"))
	err := d.Dispatch(pkt)

	if !errors.Is(err, ErrThreatScoreExceeded) {
		t.Fatalf("expected error %v, got %v", ErrThreatScoreExceeded, err)
	}
}

func TestDispatcher_IntentConstants(t *testing.T) {
	// Verify the well-known intent IDs match the spec (RFC-021).
	if !reflect.DeepEqual(byte(0x01), IntentHandshake) {
		t.Fatalf("want %v, got %v", byte(0x01), IntentHandshake)
	}
	if !reflect.DeepEqual(byte(0x20), IntentCompute) {
		t.Fatalf("want %v, got %v", byte(0x20), IntentCompute)
	}
	if !reflect.DeepEqual(byte(0x30), IntentRehab) {
		t.Fatalf("want %v, got %v", byte(0x30), IntentRehab)
	}
	if !reflect.DeepEqual(byte(0xFF), IntentCustom) {
		t.Fatalf("want %v, got %v", byte(0xFF), IntentCustom)
	}
}

// TestDispatcher_Handlers_Good verifies that the Handlers iterator yields
// every registered handler keyed by intent ID.
func TestDispatcher_Handlers_Good(t *testing.T) {
	d := NewDispatcher()

	d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error { return nil })
	d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error { return nil })
	d.RegisterHandler(IntentRehab, func(pkt *ueps.ParsedPacket) error { return nil })

	seen := make(map[byte]bool)
	for id, handler := range d.Handlers() {
		if handler == nil {
			t.Errorf("handler for intent 0x%02X is nil", id)
		}
		seen[id] = true
	}

	expected := []byte{IntentHandshake, IntentCompute, IntentRehab}
	if len(seen) != len(expected) {
		t.Errorf("expected %d handlers, got %d", len(expected), len(seen))
	}
	for _, id := range expected {
		if !seen[id] {
			t.Errorf("expected handler for intent 0x%02X in iterator output", id)
		}
	}
}

// TestDispatcher_Handlers_Bad verifies early termination and the empty-registry
// case so the iterator cannot deadlock or misreport state.
func TestDispatcher_Handlers_Bad(t *testing.T) {
	d := NewDispatcher()

	// Empty dispatcher yields nothing.
	var emptyCount int
	for range d.Handlers() {
		emptyCount++
	}
	if emptyCount != 0 {
		t.Errorf("empty dispatcher should yield 0 handlers, got %d", emptyCount)
	}

	// Early termination must stop the iterator promptly.
	d.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error { return nil })
	d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error { return nil })
	d.RegisterHandler(IntentRehab, func(pkt *ueps.ParsedPacket) error { return nil })

	var stopped int
	for range d.Handlers() {
		stopped++
		break
	}
	if stopped != 1 {
		t.Errorf("iterator should stop after first break, got %d", stopped)
	}
}
