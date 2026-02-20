package node

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufPool_GetPutRoundTrip(t *testing.T) {
	buf := getBuffer()
	require.NotNil(t, buf, "getBuffer should return a non-nil buffer")
	assert.Equal(t, 0, buf.Len(), "buffer should be empty after get")

	buf.WriteString("hello")
	assert.Equal(t, 5, buf.Len())

	putBuffer(buf)

	// Get another buffer — should be reset
	buf2 := getBuffer()
	assert.Equal(t, 0, buf2.Len(), "buffer should be reset after get")
	putBuffer(buf2)
}

func TestBufPool_BufferReuse(t *testing.T) {
	// Get a buffer, write to it, put it back, get again.
	// The pool may return the same buffer (though not guaranteed by sync.Pool).
	// We can at least verify the buffer is properly reset.
	buf1 := getBuffer()
	buf1.WriteString("reuse-test")
	cap1 := buf1.Cap()
	putBuffer(buf1)

	buf2 := getBuffer()
	assert.Equal(t, 0, buf2.Len(), "reused buffer must be reset")
	// If we got the same buffer, capacity should be at least as large
	if buf2.Cap() >= cap1 {
		// Likely the same buffer — good, it was reused
		t.Logf("buffer likely reused: cap1=%d, cap2=%d", cap1, buf2.Cap())
	}
	putBuffer(buf2)
}

func TestBufPool_LargeBufferNotPooled(t *testing.T) {
	buf := getBuffer()
	// Grow buffer beyond the 64KB threshold
	large := make([]byte, 70000)
	buf.Write(large)
	assert.Greater(t, buf.Cap(), 65536, "buffer should have grown past threshold")

	putBuffer(buf) // Should NOT be returned to the pool

	// Get a new buffer — it should be a fresh one (small capacity)
	buf2 := getBuffer()
	assert.LessOrEqual(t, buf2.Cap(), 65536,
		"buffer from pool should not be the oversized one")
	putBuffer(buf2)
}

func TestBufPool_ConcurrentGetPut(t *testing.T) {
	const goroutines = 100
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				buf := getBuffer()
				buf.WriteString("concurrent-data")
				assert.Greater(t, buf.Len(), 0)
				putBuffer(buf)
			}
		}(g)
	}

	wg.Wait()
}

func TestBufPool_BufferIndependence(t *testing.T) {
	// Get two buffers, write to one, verify the other is unaffected.
	buf1 := getBuffer()
	buf2 := getBuffer()

	buf1.WriteString("buffer-one")
	buf2.WriteString("buffer-two")

	assert.Equal(t, "buffer-one", buf1.String())
	assert.Equal(t, "buffer-two", buf2.String())

	// Writing more to buf1 should not affect buf2
	buf1.WriteString("-extra")
	assert.Equal(t, "buffer-one-extra", buf1.String())
	assert.Equal(t, "buffer-two", buf2.String())

	putBuffer(buf1)
	putBuffer(buf2)
}

func TestMarshalJSON_BasicTypes(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
		{"map", map[string]string{"key": "value"}},
		{"slice", []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pooled, err := MarshalJSON(tt.input)
			require.NoError(t, err)

			standard, err := json.Marshal(tt.input)
			require.NoError(t, err)

			assert.Equal(t, string(standard), string(pooled),
				"MarshalJSON should produce same output as json.Marshal")
		})
	}
}

func TestMarshalJSON_ReturnsIndependentCopy(t *testing.T) {
	// Ensure the returned bytes are a copy, not a reference to the pooled buffer.
	data1, err := MarshalJSON(map[string]string{"first": "call"})
	require.NoError(t, err)

	data2, err := MarshalJSON(map[string]string{"second": "call"})
	require.NoError(t, err)

	// data1 should still contain the first result, not be overwritten
	assert.True(t, bytes.Contains(data1, []byte("first")),
		"first result should be independent of second call")
	assert.True(t, bytes.Contains(data2, []byte("second")),
		"second result should contain its own data")
}

func TestMarshalJSON_NoHTMLEscaping(t *testing.T) {
	// MarshalJSON has SetEscapeHTML(false), so <, >, & should not be escaped
	data, err := MarshalJSON(map[string]string{"html": "<b>bold</b>"})
	require.NoError(t, err)

	assert.Contains(t, string(data), "<b>bold</b>",
		"HTML characters should not be escaped")
}

func TestMarshalJSON_ConcurrentCalls(t *testing.T) {
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			data, err := MarshalJSON(map[string]int{"id": id})
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		}(g)
	}

	wg.Wait()
}
