package node

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// --- bufpool.go tests ---

func TestGetBuffer_ReturnsResetBuffer(t *testing.T) {
	t.Run("buffer is initially empty", func(t *testing.T) {
		buf := getBuffer()
		defer putBuffer(buf)

		if !reflect.DeepEqual(0, buf.Len()) {
			t.Fatalf("want %v, got %v", 0, buf.Len())
		}
	})

	t.Run("buffer is reset after reuse", func(t *testing.T) {
		buf := getBuffer()
		buf.WriteString("stale data that should be cleared")
		putBuffer(buf)

		buf2 := getBuffer()
		defer putBuffer(buf2)

		if !reflect.DeepEqual(0, buf2.Len()) {
			t.Fatalf("want %v, got %v", 0, buf2.Len())
		}
	})
}

func TestBufpool_MarshalJSON_Good(t *testing.T) {
	data, err := MarshalJSON(map[string]string{"name": "node"})
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if !bytes.Contains(data, []byte(`"name":"node"`)) {
		t.Fatalf("json: %s", data)
	}
}

func TestBufpool_MarshalJSON_Bad(t *testing.T) {
	data, err := MarshalJSON(func() {})
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if data != nil {
		t.Fatalf("data: got %s, want nil", data)
	}
}

func TestBufpool_MarshalJSON_Ugly(t *testing.T) {
	data, err := MarshalJSON(map[string]string{"html": "<tag>"})
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if strings.Contains(string(data), `\u003c`) {
		t.Fatalf("expected unescaped HTML, got %s", data)
	}
}

func TestPutBuffer_DiscardsOversizedBuffers(t *testing.T) {
	t.Run("buffer at 64KB limit is pooled", func(t *testing.T) {
		buf := getBuffer()
		buf.Grow(65536)
		putBuffer(buf)

		buf2 := getBuffer()
		defer putBuffer(buf2)
		if !reflect.DeepEqual(0, buf2.Len()) {
			t.Fatalf("want %v, got %v", 0, buf2.Len())
		}
	})

	t.Run("buffer exceeding 64KB is discarded", func(t *testing.T) {
		buf := getBuffer()
		large := make([]byte, 65537)
		buf.Write(large)
		if !(buf.Cap() > 65536) {
			t.Fatalf("expected %v to be greater than %v", buf.Cap(), 65536)
		}

		putBuffer(buf)

		buf2 := getBuffer()
		defer putBuffer(buf2)
		if !(buf2.Cap() <= 65536) {
			t.Fatalf("expected %v to be less than or equal to %v", buf2.Cap(), 65536)
		}
	})
}

func TestBufPool_BufferIndependence(t *testing.T) {
	buf1 := getBuffer()
	buf2 := getBuffer()

	buf1.WriteString("buffer-one")
	buf2.WriteString("buffer-two")

	if !reflect.DeepEqual("buffer-one", buf1.String()) {
		t.Fatalf("want %v, got %v", "buffer-one", buf1.String())
	}
	if !reflect.DeepEqual("buffer-two", buf2.String()) {
		t.Fatalf("want %v, got %v", "buffer-two", buf2.String())
	}

	buf1.WriteString("-extra")
	if !reflect.DeepEqual("buffer-one-extra", buf1.String()) {
		t.Fatalf("want %v, got %v", "buffer-one-extra", buf1.String())
	}
	if !reflect.DeepEqual("buffer-two", buf2.String()) {
		t.Fatalf("want %v, got %v", "buffer-two", buf2.String())
	}

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var wantJSON1, gotJSON1 any
			if err := json.Unmarshal(expected, &wantJSON1); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := json.Unmarshal(got, &gotJSON1); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(wantJSON1, gotJSON1) {
				t.Fatalf("want JSON %s, got %s", string(expected), string(got))
			}
		})
	}
}

func TestMarshalJSON_NoTrailingNewline(t *testing.T) {
	data, err := MarshalJSON(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reflect.DeepEqual(byte('\n'), data[len(data)-1]) {
		t.Fatalf("did not want %v", data[len(data)-1])
	}
}

func TestMarshalJSON_HTMLEscaping(t *testing.T) {
	input := map[string]string{"html": "<script>alert('xss')</script>"}
	data, err := MarshalJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(data), "<script>") {
		t.Fatalf("expected %q to contain %q", string(data), "<script>")
	}
}

func TestMarshalJSON_ReturnsCopy(t *testing.T) {
	data1, err := MarshalJSON("first")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot := make([]byte, len(data1))
	copy(snapshot, data1)

	data2, err := MarshalJSON("second")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = data2

	if !reflect.DeepEqual(snapshot, data1) {
		t.Fatalf("want %v, got %v", snapshot, data1)
	}
}

func TestMarshalJSON_ReturnsIndependentCopy(t *testing.T) {
	data1, err := MarshalJSON(map[string]string{"first": "call"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data2, err := MarshalJSON(map[string]string{"second": "call"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !(bytes.Contains(data1, []byte("first"))) {
		t.Fatal("expected true")
	}
	if !(bytes.Contains(data2, []byte("second"))) {
		t.Fatal("expected true")
	}
}

func TestMarshalJSON_InvalidValue(t *testing.T) {
	ch := make(chan int)
	_, err := MarshalJSON(ch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
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

				if reflect.TypeOf(&bytes.Buffer{}) != reflect.TypeOf(buf) {
					t.Errorf("expected type %T, got %T", &bytes.Buffer{}, buf)
				}
				if !(buf.Len() > 0) {
					t.Errorf("expected %v to be greater than %v", buf.Len(), 0)
				}

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
					errs[idx] = errors.New("assertion error")
				}
			}
		}(g)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d unexpected error: %v", i, err)
		}
	}
}

func TestBufferPool_ReuseAfterReset(t *testing.T) {
	buf := getBuffer()
	buf.Write(make([]byte, 4096))
	putBuffer(buf)

	buf2 := getBuffer()
	defer putBuffer(buf2)

	if !reflect.DeepEqual(0, buf2.Len()) {
		t.Fatalf("want %v, got %v", 0, buf2.Len())
	}
	if !(buf2.Cap() >= 1024) {
		t.Fatalf("expected %v to be greater than or equal to %v", buf2.Cap(), 1024)
	}
}
