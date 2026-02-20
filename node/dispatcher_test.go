package node

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"forge.lthn.ai/core/go-p2p/ueps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		require.NoError(t, err)
		require.NotNil(t, received)
		assert.Equal(t, pkt, received)
		assert.Equal(t, []byte("hello"), received.Payload)
	})

	t.Run("handler error propagates to caller", func(t *testing.T) {
		d := NewDispatcher()
		handlerErr := fmt.Errorf("compute failed")

		d.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
			return handlerErr
		})

		pkt := makePacket(IntentCompute, 0, []byte("job"))
		err := d.Dispatch(pkt)

		assert.ErrorIs(t, err, handlerErr)
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
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.dispatched, called)
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

	assert.ErrorIs(t, err, ErrUnknownIntent)
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

			require.NoError(t, err)
			assert.True(t, *tt.want, "expected handler for intent 0x%02X to be called", tt.intentID)

			// Verify no other handler was called
			for _, other := range tests {
				if other.intentID != tt.intentID {
					assert.False(t, *other.want,
						"handler for intent 0x%02X should not have been called when dispatching 0x%02X",
						other.intentID, tt.intentID)
				}
			}
		})
	}
}

func TestDispatcher_NilAndEmptyPayload(t *testing.T) {
	t.Run("nil packet returns ErrNilPacket", func(t *testing.T) {
		d := NewDispatcher()
		err := d.Dispatch(nil)
		assert.ErrorIs(t, err, ErrNilPacket)
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

		require.NoError(t, err)
		require.NotNil(t, received)
		assert.Nil(t, received.Payload)
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

		require.NoError(t, err)
		require.NotNil(t, received)
		assert.Empty(t, received.Payload)
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

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			pkt := makePacket(IntentCompute, 0, []byte("concurrent"))
			err := d.Dispatch(pkt)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(goroutines), count.Load())
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
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			pkt := makePacket(IntentHandshake, 0, []byte("data"))
			_ = d.Dispatch(pkt)
		}()
	}

	// Half the goroutines register/replace handlers concurrently
	for i := 0; i < goroutines; i++ {
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
	assert.True(t, count.Load() >= 0)
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

	require.NoError(t, err)
	assert.False(t, firstCalled, "original handler should not be called after replacement")
	assert.True(t, secondCalled, "replacement handler should be called")
}

func TestDispatcher_ThreatBlocksBeforeRouting(t *testing.T) {
	// Verify that the circuit breaker fires before intent routing,
	// so even an unknown intent returns ErrThreatScoreExceeded (not ErrUnknownIntent).
	d := NewDispatcher()

	pkt := makePacket(0x42, ThreatScoreThreshold+1, []byte("hostile"))
	err := d.Dispatch(pkt)

	assert.ErrorIs(t, err, ErrThreatScoreExceeded,
		"threat circuit breaker should fire before intent routing")
}

func TestDispatcher_IntentConstants(t *testing.T) {
	// Verify the well-known intent IDs match the spec (RFC-021).
	assert.Equal(t, byte(0x01), IntentHandshake)
	assert.Equal(t, byte(0x20), IntentCompute)
	assert.Equal(t, byte(0x30), IntentRehab)
	assert.Equal(t, byte(0xFF), IntentCustom)
}
