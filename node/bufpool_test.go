package node

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- bufpool.go tests ---

func TestGetBuffer_ReturnsResetBuffer(t *testing.T) {
	t.Run("buffer is initially empty", func(t *testing.T) {
		buf := getBuffer()
		defer putBuffer(buf)

		assert.Equal(t, 0, buf.Len(), "buffer from pool should have zero length")
	})

	t.Run("buffer is reset after reuse", func(t *testing.T) {
		buf := getBuffer()
		buf.WriteString("stale data that should be cleared")
		putBuffer(buf)

		buf2 := getBuffer()
		defer putBuffer(buf2)

		assert.Equal(t, 0, buf2.Len(),
			"reused buffer should be reset (no stale data)")
	})
}

func TestPutBuffer_DiscardsOversizedBuffers(t *testing.T) {
	t.Run("buffer at 64KB limit is pooled", func(t *testing.T) {
		buf := getBuffer()
		buf.Grow(65536)
		putBuffer(buf)

		buf2 := getBuffer()
		defer putBuffer(buf2)
		assert.Equal(t, 0, buf2.Len())
	})

	t.Run("buffer exceeding 64KB is discarded", func(t *testing.T) {
		buf := getBuffer()
		large := make([]byte, 65537)
		buf.Write(large)
		assert.Greater(t, buf.Cap(), 65536, "buffer should have grown past 64KB")

		putBuffer(buf)

		buf2 := getBuffer()
		defer putBuffer(buf2)
		assert.LessOrEqual(t, buf2.Cap(), 65536,
			"pool should not return an oversized buffer")
	})
}

func TestBufPool_BufferIndependence(t *testing.T) {
	buf1 := getBuffer()
	buf2 := getBuffer()

	buf1.WriteString("buffer-one")
	buf2.WriteString("buffer-two")

	assert.Equal(t, "buffer-one", buf1.String())
	assert.Equal(t, "buffer-two", buf2.String())

	buf1.WriteString("-extra")
	assert.Equal(t, "buffer-one-extra", buf1.String())
	assert.Equal(t, "buffer-two", buf2.String())

	putBuffer(buf1)
	putBuffer(buf2)
}

func TestMarshalJSON_BasicTypes(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name:  "string value",
			input: "hello",
		},
		{
			name:  "integer value",
			input: 42,
		},
		{
			name:  "float value",
			input: 3.14,
		},
		{
			name:  "boolean value",
			input: true,
		},
		{
			name:  "nil value",
			input: nil,
		},
		{
			name:  "struct value",
			input: PingPayload{SentAt: 1234567890},
		},
		{
			name:  "map value",
			input: map[string]any{"key": "value", "num": 42},
		},
		{
			name:  "slice value",
			input: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MarshalJSON(tt.input)
			require.NoError(t, err)

			expected, err := json.Marshal(tt.input)
			require.NoError(t, err)

			assert.JSONEq(t, string(expected), string(got),
				"MarshalJSON output should match json.Marshal")
		})
	}
}

func TestMarshalJSON_NoTrailingNewline(t *testing.T) {
	data, err := MarshalJSON(map[string]string{"key": "value"})
	require.NoError(t, err)

	assert.NotEqual(t, byte('\n'), data[len(data)-1],
		"MarshalJSON should strip the trailing newline added by json.Encoder")
}

func TestMarshalJSON_HTMLEscaping(t *testing.T) {
	input := map[string]string{"html": "<script>alert('xss')</script>"}
	data, err := MarshalJSON(input)
	require.NoError(t, err)

	assert.Contains(t, string(data), "<script>",
		"HTML characters should not be escaped when EscapeHTML is false")
}

func TestMarshalJSON_ReturnsCopy(t *testing.T) {
	data1, err := MarshalJSON("first")
	require.NoError(t, err)

	snapshot := make([]byte, len(data1))
	copy(snapshot, data1)

	data2, err := MarshalJSON("second")
	require.NoError(t, err)
	_ = data2

	assert.Equal(t, snapshot, data1,
		"returned slice should be a copy and not be mutated by subsequent calls")
}

func TestMarshalJSON_ReturnsIndependentCopy(t *testing.T) {
	data1, err := MarshalJSON(map[string]string{"first": "call"})
	require.NoError(t, err)

	data2, err := MarshalJSON(map[string]string{"second": "call"})
	require.NoError(t, err)

	assert.True(t, bytes.Contains(data1, []byte("first")),
		"first result should be independent of second call")
	assert.True(t, bytes.Contains(data2, []byte("second")),
		"second result should contain its own data")
}

func TestMarshalJSON_InvalidValue(t *testing.T) {
	ch := make(chan int)
	_, err := MarshalJSON(ch)
	assert.Error(t, err, "marshalling an unserialisable type should return an error")
}

func TestBufferPool_ConcurrentAccess(t *testing.T) {
	const goroutines = 100
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				buf := getBuffer()
				buf.WriteString("concurrent test data")

				assert.IsType(t, &bytes.Buffer{}, buf)
				assert.Greater(t, buf.Len(), 0)

				putBuffer(buf)
			}
		}()
	}

	wg.Wait()
}

func TestMarshalJSON_ConcurrentSafety(t *testing.T) {
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make([]error, goroutines)

	for g := range goroutines {
		go func(idx int) {
			defer wg.Done()
			payload := PingPayload{SentAt: int64(idx)}
			data, err := MarshalJSON(payload)
			errs[idx] = err

			if err == nil {
				var parsed PingPayload
				err = json.Unmarshal(data, &parsed)
				if err != nil {
					errs[idx] = err
					return
				}
				if parsed.SentAt != int64(idx) {
					errs[idx] = assert.AnError
				}
			}
		}(g)
	}

	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d should not produce an error", i)
	}
}

func TestBufferPool_ReuseAfterReset(t *testing.T) {
	buf := getBuffer()
	buf.Write(make([]byte, 4096))
	putBuffer(buf)

	buf2 := getBuffer()
	defer putBuffer(buf2)

	assert.Equal(t, 0, buf2.Len(), "buffer should be reset")
	assert.GreaterOrEqual(t, buf2.Cap(), 1024,
		"buffer capacity should be at least the default (1024)")
}
