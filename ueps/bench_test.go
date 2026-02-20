package ueps

import (
	"bufio"
	"bytes"
	"testing"
)

// benchSecret is a deterministic shared secret for reproducible benchmarks.
var benchSecret = []byte("bench-shared-secret-32-bytes!!!!")

// BenchmarkPacketBuild measures UEPS PacketBuilder marshal + HMAC signing.
func BenchmarkPacketBuild(b *testing.B) {
	payload := bytes.Repeat([]byte("A"), 256)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		builder := NewBuilder(0x20, payload)
		_, err := builder.MarshalAndSign(benchSecret)
		if err != nil {
			b.Fatalf("MarshalAndSign: %v", err)
		}
	}
}

// BenchmarkPacketRead measures UEPS ReadAndVerify (unmarshal + HMAC verification).
func BenchmarkPacketRead(b *testing.B) {
	payload := bytes.Repeat([]byte("B"), 256)
	builder := NewBuilder(0x20, payload)
	frame, err := builder.MarshalAndSign(benchSecret)
	if err != nil {
		b.Fatalf("MarshalAndSign: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		reader := bufio.NewReader(bytes.NewReader(frame))
		_, err := ReadAndVerify(reader, benchSecret)
		if err != nil {
			b.Fatalf("ReadAndVerify: %v", err)
		}
	}
}

// BenchmarkPacketRoundTrip measures full build + read + verify cycle.
func BenchmarkPacketRoundTrip(b *testing.B) {
	payload := []byte("round-trip benchmark payload data")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		builder := NewBuilder(0x01, payload)
		frame, err := builder.MarshalAndSign(benchSecret)
		if err != nil {
			b.Fatalf("MarshalAndSign: %v", err)
		}

		_, err = ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), benchSecret)
		if err != nil {
			b.Fatalf("ReadAndVerify: %v", err)
		}
	}
}

// BenchmarkPacketBuild_LargePayload measures marshalling with a 4KB payload.
func BenchmarkPacketBuild_LargePayload(b *testing.B) {
	payload := bytes.Repeat([]byte("X"), 4096)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		builder := NewBuilder(0xFF, payload)
		_, err := builder.MarshalAndSign(benchSecret)
		if err != nil {
			b.Fatalf("MarshalAndSign: %v", err)
		}
	}
}

// BenchmarkPacketRead_LargePayload measures reading/verifying a 4KB payload.
func BenchmarkPacketRead_LargePayload(b *testing.B) {
	payload := bytes.Repeat([]byte("Y"), 4096)
	builder := NewBuilder(0xFF, payload)
	frame, err := builder.MarshalAndSign(benchSecret)
	if err != nil {
		b.Fatalf("MarshalAndSign: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), benchSecret)
		if err != nil {
			b.Fatalf("ReadAndVerify: %v", err)
		}
	}
}

// BenchmarkPacketBuild_EmptyPayload measures overhead with zero-length payload.
func BenchmarkPacketBuild_EmptyPayload(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		builder := NewBuilder(0x01, nil)
		_, err := builder.MarshalAndSign(benchSecret)
		if err != nil {
			b.Fatalf("MarshalAndSign: %v", err)
		}
	}
}
