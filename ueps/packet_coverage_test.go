package ueps

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"testing"
)

// failWriter returns an error after n successful Write calls.
// Used to exercise every error branch inside writeTLV.
type failWriter struct {
	remaining int
}

func (f *failWriter) Write(p []byte) (int, error) {
	if f.remaining <= 0 {
		return 0, errors.New("write failed")
	}
	f.remaining--
	return len(p), nil
}

// TestWriteTLV_TagWriteFails verifies writeTLV returns an error
// when the very first Write (the tag byte) fails.
func TestWriteTLV_TagWriteFails(t *testing.T) {
	w := &failWriter{remaining: 0}
	err := writeTLV(w, TagVersion, []byte{0x09})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !reflect.DeepEqual("write failed", err.Error()) {
		t.Fatalf("want %v, got %v", "write failed", err.Error())
	}
}

// TestWriteTLV_LengthWriteFails verifies writeTLV returns an error
// when the second Write (the length byte) fails.
func TestWriteTLV_LengthWriteFails(t *testing.T) {
	w := &failWriter{remaining: 1}
	err := writeTLV(w, TagVersion, []byte{0x09})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !reflect.DeepEqual("write failed", err.Error()) {
		t.Fatalf("want %v, got %v", "write failed", err.Error())
	}
}

// TestWriteTLV_ValueWriteFails verifies writeTLV returns an error
// when the third Write (the value bytes) fails.
func TestWriteTLV_ValueWriteFails(t *testing.T) {
	w := &failWriter{remaining: 2}
	err := writeTLV(w, TagVersion, []byte{0x09})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !reflect.DeepEqual("write failed", err.Error()) {
		t.Fatalf("want %v, got %v", "write failed", err.Error())
	}
}

// errorAfterNReader delivers a fixed prefix of valid bytes then
// returns an error on any subsequent read. This lets us exercise
// the io.ReadAll failure path in ReadAndVerify (reader.go:51-53).
type errorAfterNReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errorAfterNReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	// If we have exhausted the buffer mid-read, return what we have;
	// the next call will surface the error.
	return n, nil
}

// TestReadAndVerify_PayloadReadError exercises the error branch at
// reader.go:51-53 where io.ReadAll fails after the 0xFF tag byte
// has been successfully read.
func TestReadAndVerify_PayloadReadError(t *testing.T) {
	// Build a valid packet so we have genuine TLV headers + HMAC.
	payload := []byte("coverage test")
	builder := NewBuilder(0x20, payload)
	frame, err := builder.MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the position of the 0xFF (TagPayload) byte in the frame.
	// Everything up to and including 0xFF will be delivered; the
	// payload bytes that follow will be replaced by an I/O error.
	payloadTagIdx := -1
	for i, b := range frame {
		if b == TagPayload {
			payloadTagIdx = i
			break
		}
	}
	if reflect.DeepEqual(-1, payloadTagIdx) {
		t.Fatalf("did not want %v", payloadTagIdx)
	}

	// Deliver bytes up to and including the 0xFF tag, then error.
	prefix := frame[:payloadTagIdx+1]
	r := &errorAfterNReader{
		data: prefix,
		err:  errors.New("connection reset"),
	}

	_, err = ReadAndVerify(bufio.NewReader(r), testSecret)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !reflect.DeepEqual("connection reset", err.Error()) {
		t.Fatalf("want %v, got %v", "connection reset", err.Error())
	}
}

// TestReadAndVerify_PayloadReadError_EOF ensures that a truncated payload
// (missing bytes after TagPayload) is handled as an I/O error (UnexpectedEOF)
// because ReadAndVerify now uses io.ReadFull with the expected length prefix.
func TestReadAndVerify_PayloadReadError_EOF(t *testing.T) {
	payload := []byte("eof test")
	builder := NewBuilder(0x20, payload)
	frame, err := builder.MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Truncate at TagPayload tag + partial length — the reader will see 0xFF
	// then EOF while trying to read the 2-byte length or the payload itself.
	payloadTagIdx := bytes.IndexByte(frame, TagPayload)
	if reflect.DeepEqual(-1, payloadTagIdx) {
		t.Fatalf("did not want %v", payloadTagIdx)
	}

	truncated := frame[:payloadTagIdx+1] // Only the tag, no length
	_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(truncated)), testSecret)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected error %v, got %v", io.EOF, err)
	} // Failed reading length

	truncatedWithLen := frame[:payloadTagIdx+3] // Tag + Length, but no payload
	_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(truncatedWithLen)), testSecret)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// io.ReadFull returns io.EOF if no bytes are read at all before EOF.
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected error %v, got %v", io.EOF, err)
	}
}

// TestWriteTLV_AllWritesSucceed confirms the happy path still works
// after exercising all error branches — a simple sanity check using
// failWriter with enough remaining writes.
func TestWriteTLV_AllWritesSucceed(t *testing.T) {
	var buf bytes.Buffer
	err := writeTLV(&buf, TagVersion, []byte{0x09})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Now uses 2-byte big-endian length: 0x00 0x01
	if !reflect.DeepEqual([]byte{TagVersion, 0x00, 0x01, 0x09}, buf.Bytes()) {
		t.Fatalf("want %v, got %v", []byte{TagVersion, 0x00, 0x01, 0x09}, buf.Bytes())
	}
}

// TestWriteTLV_FailWriterTable runs the three failure scenarios in
// a table-driven fashion for completeness.
func TestWriteTLV_FailWriterTable(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		failsAt   string
	}{
		{"TagWriteFails", 0, "tag"},
		{"LengthWriteFails", 1, "length"},
		{"ValueWriteFails", 2, "value"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &failWriter{remaining: tc.remaining}
			err := writeTLV(w, TagIntent, []byte{0x42})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestReadAndVerify_ManualPacket_PayloadReadError builds the packet
// entirely by hand (no MarshalAndSign) so we can validate the exact
// HMAC computation independently of the builder. This also serves as
// a cross-check that our errorAfterNReader is not accidentally
// corrupting the prefix bytes.
func TestReadAndVerify_ManualPacket_PayloadReadError(t *testing.T) {
	payload := []byte("manual test")

	// Build header TLVs
	var hdr bytes.Buffer
	if err := writeTLV(&hdr, TagVersion, []byte{0x09}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := writeTLV(&hdr, TagCurrentLay, []byte{5}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := writeTLV(&hdr, TagTargetLay, []byte{5}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := writeTLV(&hdr, TagIntent, []byte{0x20}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tsBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(tsBuf, 0)
	if err := writeTLV(&hdr, TagThreatScore, tsBuf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Compute HMAC
	mac := hmac.New(sha256.New, testSecret)
	mac.Write(hdr.Bytes())
	mac.Write(payload)
	sig := mac.Sum(nil)

	// Assemble full frame up to (and including) 0xFF tag
	var frame bytes.Buffer
	frame.Write(hdr.Bytes())
	if err := writeTLV(&frame, TagHMAC, sig); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frame.WriteByte(TagPayload)
	// Do NOT write payload — the errorAfterNReader will inject an error here.

	r := &errorAfterNReader{
		data: frame.Bytes(),
		err:  io.ErrUnexpectedEOF,
	}

	_, err := ReadAndVerify(bufio.NewReader(r), testSecret)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !reflect.DeepEqual(io.ErrUnexpectedEOF, err) {
		t.Fatalf("want %v, got %v", io.ErrUnexpectedEOF, err)
	}
}
