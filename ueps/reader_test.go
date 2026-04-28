package ueps

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestReader_ReadAndVerify_Good(t *testing.T) {
	frame, err := NewBuilder(0x20, []byte("payload")).MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("MarshalAndSign: %v", err)
	}
	parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), testSecret)
	if err != nil {
		t.Fatalf("ReadAndVerify: %v", err)
	}
	if !bytes.Equal(parsed.Payload, []byte("payload")) {
		t.Fatalf("payload: got %q", parsed.Payload)
	}
}

func TestReader_ReadAndVerify_Bad(t *testing.T) {
	frame, err := NewBuilder(0x20, []byte("payload")).MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("MarshalAndSign: %v", err)
	}
	_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), []byte("wrong-secret"))
	if err == nil {
		t.Fatal("expected HMAC mismatch")
	}
	if !strings.Contains(err.Error(), "integrity violation") {
		t.Fatalf("error: got %v", err)
	}
}

func TestReader_ReadAndVerify_Ugly(t *testing.T) {
	parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(nil)), testSecret)
	if err != io.EOF {
		t.Fatalf("error: got %v, want EOF", err)
	}
	if parsed != nil {
		t.Fatalf("parsed: got %#v, want nil", parsed)
	}
}
