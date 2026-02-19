package ueps

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

// testSecret is a deterministic shared secret for reproducible tests.
var testSecret = []byte("test-shared-secret-32-bytes!!!!!")

func TestPacketBuilder_RoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		intentID    uint8
		payload     []byte
		threatScore uint16
	}{
		{
			name:     "BasicPayload",
			intentID: 0x20,
			payload:  []byte("hello UEPS"),
		},
		{
			name:     "BinaryPayload",
			intentID: 0x01,
			payload:  []byte{0x00, 0xFF, 0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:        "WithThreatScore",
			intentID:    0x30,
			payload:     []byte("threat test"),
			threatScore: 42,
		},
		{
			name:     "LargePayload",
			intentID: 0xFF,
			payload:  bytes.Repeat([]byte("A"), 4096),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewBuilder(tc.intentID, tc.payload)
			builder.Header.ThreatScore = tc.threatScore

			frame, err := builder.MarshalAndSign(testSecret)
			if err != nil {
				t.Fatalf("MarshalAndSign failed: %v", err)
			}

			parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), testSecret)
			if err != nil {
				t.Fatalf("ReadAndVerify failed: %v", err)
			}

			// Verify all header fields round-trip correctly
			if parsed.Header.Version != 0x09 {
				t.Errorf("Version: got 0x%02x, want 0x09", parsed.Header.Version)
			}
			if parsed.Header.CurrentLayer != 5 {
				t.Errorf("CurrentLayer: got %d, want 5", parsed.Header.CurrentLayer)
			}
			if parsed.Header.TargetLayer != 5 {
				t.Errorf("TargetLayer: got %d, want 5", parsed.Header.TargetLayer)
			}
			if parsed.Header.IntentID != tc.intentID {
				t.Errorf("IntentID: got 0x%02x, want 0x%02x", parsed.Header.IntentID, tc.intentID)
			}
			if parsed.Header.ThreatScore != tc.threatScore {
				t.Errorf("ThreatScore: got %d, want %d", parsed.Header.ThreatScore, tc.threatScore)
			}

			// Verify payload integrity
			if !bytes.Equal(parsed.Payload, tc.payload) {
				t.Errorf("Payload mismatch: got %d bytes, want %d bytes", len(parsed.Payload), len(tc.payload))
			}
		})
	}
}

func TestHMACVerification_TamperedPayload(t *testing.T) {
	builder := NewBuilder(0x20, []byte("original payload"))
	frame, err := builder.MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("MarshalAndSign failed: %v", err)
	}

	// Flip the last byte of the frame (which is in the payload)
	tampered := make([]byte, len(frame))
	copy(tampered, frame)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(tampered)), testSecret)
	if err == nil {
		t.Fatal("Expected HMAC mismatch error for tampered payload")
	}
	if !strings.Contains(err.Error(), "integrity violation") {
		t.Errorf("Expected integrity violation error, got: %v", err)
	}
}

func TestHMACVerification_TamperedHeader(t *testing.T) {
	builder := NewBuilder(0x20, []byte("test payload"))
	frame, err := builder.MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("MarshalAndSign failed: %v", err)
	}

	// Tamper with the Version TLV value (byte index 2: tag=0, len=1, val=2)
	tampered := make([]byte, len(frame))
	copy(tampered, frame)
	tampered[2] = 0x01 // Change version from 0x09 to 0x01

	_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(tampered)), testSecret)
	if err == nil {
		t.Fatal("Expected HMAC mismatch error for tampered header")
	}
	if !strings.Contains(err.Error(), "integrity violation") {
		t.Errorf("Expected integrity violation error, got: %v", err)
	}
}

func TestHMACVerification_WrongSharedSecret(t *testing.T) {
	builder := NewBuilder(0x20, []byte("secret data"))
	frame, err := builder.MarshalAndSign([]byte("key-A-used-for-signing!!!!!!!!!!"))
	if err != nil {
		t.Fatalf("MarshalAndSign failed: %v", err)
	}

	_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), []byte("key-B-used-for-reading!!!!!!!!!!"))
	if err == nil {
		t.Fatal("Expected HMAC mismatch error for wrong shared secret")
	}
	if !strings.Contains(err.Error(), "integrity violation") {
		t.Errorf("Expected integrity violation error, got: %v", err)
	}
}

func TestEmptyPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"NilPayload", nil},
		{"EmptySlice", []byte{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewBuilder(0x01, tc.payload)
			frame, err := builder.MarshalAndSign(testSecret)
			if err != nil {
				t.Fatalf("MarshalAndSign failed: %v", err)
			}

			parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), testSecret)
			if err != nil {
				t.Fatalf("ReadAndVerify failed: %v", err)
			}

			if len(parsed.Payload) != 0 {
				t.Errorf("Expected empty payload, got %d bytes", len(parsed.Payload))
			}
			if parsed.Header.IntentID != 0x01 {
				t.Errorf("IntentID: got 0x%02x, want 0x01", parsed.Header.IntentID)
			}
		})
	}
}

func TestMaxThreatScoreBoundary(t *testing.T) {
	builder := NewBuilder(0x20, []byte("threat boundary"))
	builder.Header.ThreatScore = 65535 // uint16 max

	frame, err := builder.MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("MarshalAndSign failed: %v", err)
	}

	parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), testSecret)
	if err != nil {
		t.Fatalf("ReadAndVerify failed: %v", err)
	}

	if parsed.Header.ThreatScore != 65535 {
		t.Errorf("ThreatScore: got %d, want 65535", parsed.Header.ThreatScore)
	}
}

func TestMissingHMACTag(t *testing.T) {
	// Craft a packet manually: header TLVs + payload tag, but no HMAC (0x06)
	var buf bytes.Buffer

	// Write header TLVs
	writeTLV(&buf, TagVersion, []byte{0x09})
	writeTLV(&buf, TagCurrentLay, []byte{5})
	writeTLV(&buf, TagTargetLay, []byte{5})
	writeTLV(&buf, TagIntent, []byte{0x20})
	tsBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(tsBuf, 0)
	writeTLV(&buf, TagThreatScore, tsBuf)

	// Skip HMAC TLV entirely — go straight to payload
	buf.WriteByte(TagPayload)
	buf.Write([]byte("some data"))

	_, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(buf.Bytes())), testSecret)
	if err == nil {
		t.Fatal("Expected 'missing HMAC' error")
	}
	if !strings.Contains(err.Error(), "missing HMAC") {
		t.Errorf("Expected 'missing HMAC' error, got: %v", err)
	}
}

func TestWriteTLV_ValueTooLarge(t *testing.T) {
	var buf bytes.Buffer
	oversized := make([]byte, 256) // 1 byte over the 255 limit
	err := writeTLV(&buf, TagVersion, oversized)
	if err == nil {
		t.Fatal("Expected error for TLV value > 255 bytes")
	}
	if !strings.Contains(err.Error(), "TLV value too large") {
		t.Errorf("Expected 'TLV value too large' error, got: %v", err)
	}
}

func TestTruncatedPacket(t *testing.T) {
	builder := NewBuilder(0x20, []byte("full payload"))
	frame, err := builder.MarshalAndSign(testSecret)
	if err != nil {
		t.Fatalf("MarshalAndSign failed: %v", err)
	}

	tests := []struct {
		name    string
		cutAt   int
		wantErr string
	}{
		{
			name:    "CutInFirstTLVHeader",
			cutAt:   1, // Only tag byte, no length
			wantErr: "EOF",
		},
		{
			name:    "CutInFirstTLVValue",
			cutAt:   2, // Tag + length, but missing value
			wantErr: "EOF",
		},
		{
			name:    "CutMidHMAC",
			cutAt:   20, // Somewhere inside the header TLVs or HMAC
			wantErr: "",  // Any io error
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			truncated := frame[:tc.cutAt]
			_, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(truncated)), testSecret)
			if err == nil {
				t.Fatal("Expected error for truncated packet")
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestUnknownTLVTag(t *testing.T) {
	// Build a valid packet, then inject an unknown tag before the HMAC.
	// The unknown tag must be included in signedData for HMAC to pass.
	payload := []byte("tagged payload")

	// Manually construct headers + unknown tag + HMAC + payload
	var headerBuf bytes.Buffer

	// Standard header TLVs
	writeTLV(&headerBuf, TagVersion, []byte{0x09})
	writeTLV(&headerBuf, TagCurrentLay, []byte{5})
	writeTLV(&headerBuf, TagTargetLay, []byte{5})
	writeTLV(&headerBuf, TagIntent, []byte{0x20})
	tsBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(tsBuf, 0)
	writeTLV(&headerBuf, TagThreatScore, tsBuf)

	// Unknown tag (0xAA) with some data
	unknownValue := []byte{0xDE, 0xAD}
	writeTLV(&headerBuf, 0xAA, unknownValue)

	// Compute HMAC over (all header TLVs including unknown + payload)
	mac := hmac.New(sha256.New, testSecret)
	mac.Write(headerBuf.Bytes())
	mac.Write(payload)
	signature := mac.Sum(nil)

	// Assemble full frame: headers + unknown + HMAC TLV + 0xFF + payload
	var frame bytes.Buffer
	frame.Write(headerBuf.Bytes())
	writeTLV(&frame, TagHMAC, signature)
	frame.WriteByte(TagPayload)
	frame.Write(payload)

	parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame.Bytes())), testSecret)
	if err != nil {
		t.Fatalf("ReadAndVerify should accept unknown tag: %v", err)
	}

	// Header fields should still be correct
	if parsed.Header.Version != 0x09 {
		t.Errorf("Version: got 0x%02x, want 0x09", parsed.Header.Version)
	}
	if parsed.Header.IntentID != 0x20 {
		t.Errorf("IntentID: got 0x%02x, want 0x20", parsed.Header.IntentID)
	}
	if !bytes.Equal(parsed.Payload, payload) {
		t.Errorf("Payload mismatch")
	}
}

func TestNewBuilder_Defaults(t *testing.T) {
	builder := NewBuilder(0x20, []byte("data"))

	if builder.Header.Version != 0x09 {
		t.Errorf("Default Version: got 0x%02x, want 0x09", builder.Header.Version)
	}
	if builder.Header.CurrentLayer != 5 {
		t.Errorf("Default CurrentLayer: got %d, want 5", builder.Header.CurrentLayer)
	}
	if builder.Header.TargetLayer != 5 {
		t.Errorf("Default TargetLayer: got %d, want 5", builder.Header.TargetLayer)
	}
	if builder.Header.ThreatScore != 0 {
		t.Errorf("Default ThreatScore: got %d, want 0", builder.Header.ThreatScore)
	}
	if builder.Header.IntentID != 0x20 {
		t.Errorf("IntentID: got 0x%02x, want 0x20", builder.Header.IntentID)
	}
}

func TestThreatScoreBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		score uint16
	}{
		{"Zero", 0},
		{"One", 1},
		{"Mid", 32768},
		{"Max", 65535},
		{"HighThreat", 50001},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewBuilder(0x20, []byte("score test"))
			builder.Header.ThreatScore = tc.score

			frame, err := builder.MarshalAndSign(testSecret)
			if err != nil {
				t.Fatalf("MarshalAndSign failed: %v", err)
			}

			parsed, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), testSecret)
			if err != nil {
				t.Fatalf("ReadAndVerify failed: %v", err)
			}

			if parsed.Header.ThreatScore != tc.score {
				t.Errorf("ThreatScore: got %d, want %d", parsed.Header.ThreatScore, tc.score)
			}
		})
	}
}

func TestWriteTLV_BoundaryLengths(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"Empty", 0, false},
		{"OneByte", 1, false},
		{"MaxValid", 255, false},
		{"OneOver", 256, true},
		{"WayOver", 1024, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			value := make([]byte, tc.length)
			err := writeTLV(&buf, 0x01, value)
			if tc.wantErr && err == nil {
				t.Error("Expected error for oversized TLV value")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestReadAndVerify_EmptyReader verifies behaviour on completely empty input.
func TestReadAndVerify_EmptyReader(t *testing.T) {
	_, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(nil)), testSecret)
	if err == nil {
		t.Fatal("Expected error for empty reader")
	}
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got: %v", err)
	}
}
