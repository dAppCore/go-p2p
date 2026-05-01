package ueps

import (
	"bufio"
	core "dappco.re/go"
	"io"
	"testing"
)

func TestReader_ReadAndVerify_Good(t *testing.T) {
	frame, err := uepsResultValue[[]byte](NewBuilder(0x20, []byte("payload")).MarshalAndSign(testSecret))
	if err != nil {
		t.Fatalf("MarshalAndSign: %v", err)
	}
	parsed, err := uepsResultValue[*ParsedPacket](ReadAndVerify(bufio.NewReader(core.NewBuffer(frame)), testSecret))
	if err != nil {
		t.Fatalf("ReadAndVerify: %v", err)
	}
	if !core.DeepEqual(parsed.Payload, []byte("payload")) {
		t.Fatalf("payload: got %q", parsed.Payload)
	}
}

func TestReader_ReadAndVerify_Bad(t *testing.T) {
	frame, err := uepsResultValue[[]byte](NewBuilder(0x20, []byte("payload")).MarshalAndSign(testSecret))
	if err != nil {
		t.Fatalf("MarshalAndSign: %v", err)
	}
	_, err = uepsResultValue[*ParsedPacket](ReadAndVerify(bufio.NewReader(core.NewBuffer(frame)), []byte("wrong-secret")))
	if err == nil {
		t.Fatal("expected HMAC mismatch")
	}
	if !core.Contains(err.Error(), "integrity violation") {
		t.Fatalf("error: got %v", err)
	}
}

func TestReader_ReadAndVerify_Ugly(t *testing.T) {
	parsed, err := uepsResultValue[*ParsedPacket](ReadAndVerify(bufio.NewReader(core.NewBuffer(nil)), testSecret))
	if err != io.EOF {
		t.Fatalf("error: got %v, want EOF", err)
	}
	if parsed != nil {
		t.Fatalf("parsed: got %#v, want nil", parsed)
	}
}
