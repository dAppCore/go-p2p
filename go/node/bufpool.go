package node

import (
	"sync"

	core "dappco.re/go"
)

type pooledBuffer interface {
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	Bytes() []byte
	Len() int
	Cap() int
	Reset()
	Grow(int)
	String() string
}

// bufferPool provides reusable byte buffers for JSON encoding.
// This reduces allocation overhead in hot paths like message serialization.
var bufferPool = sync.Pool{
	New: func() any {
		return core.NewBuffer(make([]byte, 0, 1024))
	},
}

// getBuffer retrieves a buffer from the pool.
func getBuffer() pooledBuffer {
	buf := bufferPool.Get().(pooledBuffer)
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool.
func putBuffer(buf pooledBuffer) {
	// Don't pool buffers that grew too large (>64KB)
	if buf.Cap() <= 65536 {
		bufferPool.Put(buf)
	}
}

// MarshalJSON encodes a value to JSON using a pooled buffer.
// Returns a copy of the encoded bytes (safe to use after the function returns).
func MarshalJSON(v any) core.Result {
	if raw, ok := v.(RawMessage); ok {
		return raw.MarshalRawJSON()
	}
	if raw, ok := v.(*RawMessage); ok {
		if raw == nil {
			return core.Fail(core.NewError("node.RawMessage: marshal on nil pointer"))
		}
		return raw.MarshalRawJSON()
	}
	if msg, ok := v.(*Message); ok {
		return marshalMessageJSON(msg)
	}
	if msg, ok := v.(Message); ok {
		return marshalMessageJSON(&msg)
	}

	r := core.JSONMarshal(v)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return core.Fail(err)
		}
		return core.Fail(core.NewError("json marshal failed"))
	}

	text := string(r.Value.([]byte))
	text = core.Replace(text, `\u003c`, "<")
	text = core.Replace(text, `\u003e`, ">")
	text = core.Replace(text, `\u0026`, "&")
	data := []byte(text)

	result := make([]byte, len(data))
	copy(result, data)
	return core.Ok(result)
}
