// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"errors"
	"reflect"
	"testing"
)

func TestPackVarint_Value5(t *testing.T) {
	// 5 << 2 | 0x00 = 20 = 0x14
	got := PackVarint(5)
	if !reflect.DeepEqual([]byte{0x14}, got) {
		t.Fatalf("want %v, got %v", []byte{0x14}, got)
	}
}

func TestPackVarint_Value100(t *testing.T) {
	// 100 << 2 | 0x01 = 401 = 0x0191 → LE [0x91, 0x01]
	got := PackVarint(100)
	if !reflect.DeepEqual([]byte{0x91, 0x01}, got) {
		t.Fatalf("want %v, got %v", []byte{0x91, 0x01}, got)
	}
}

func TestPackVarint_Value65536(t *testing.T) {
	// 65536 << 2 | 0x02 = 262146 = 0x00040002 → LE [0x02, 0x00, 0x04, 0x00]
	got := PackVarint(65536)
	if !reflect.DeepEqual([]byte{0x02, 0x00, 0x04, 0x00}, got) {
		t.Fatalf("want %v, got %v", []byte{0x02, 0x00, 0x04, 0x00}, got)
	}
}

func TestPackVarint_Value2Billion(t *testing.T) {
	got := PackVarint(2_000_000_000)
	if len(got) != 8 {
		t.Fatalf("want len %v, got %v", 8, len(got))
	}
	// Low 2 bits must be 0x03 (8-byte mark).
	if !reflect.DeepEqual(byte(0x03), got[0]&0x03) {
		t.Fatalf("want %v, got %v", byte(0x03), got[0]&0x03)
	}
}

func TestPackVarint_Zero(t *testing.T) {
	got := PackVarint(0)
	if !reflect.DeepEqual([]byte{0x00}, got) {
		t.Fatalf("want %v, got %v", []byte{0x00}, got)
	}
}

func TestPackVarint_Boundaries(t *testing.T) {
	tests := []struct {
		name    string
		value   uint64
		wantLen int
	}{
		{"1-byte max (63)", 63, 1},
		{"2-byte min (64)", 64, 2},
		{"2-byte max (16383)", 16_383, 2},
		{"4-byte min (16384)", 16_384, 4},
		{"4-byte max (1073741823)", 1_073_741_823, 4},
		{"8-byte min (1073741824)", 1_073_741_824, 8},
		{"8-byte max", 4_611_686_018_427_387_903, 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PackVarint(tc.value)
			if len(got) != tc.wantLen {
				t.Fatalf("want len %v, got %v", tc.wantLen, len(got))
			}
		})
	}
}

func TestVarint_RoundTrip(t *testing.T) {
	values := []uint64{
		0, 1, 63, 64, 100, 16_383, 16_384,
		1_073_741_823, 1_073_741_824,
		4_611_686_018_427_387_903,
	}

	for _, v := range values {
		buf := PackVarint(v)
		decoded, consumed, err := UnpackVarint(buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(v, decoded) {
			t.Fatalf("want %v, got %v", v, decoded)
		}
		if !reflect.DeepEqual(len(buf), consumed) {
			t.Fatalf("want %v, got %v", len(buf), consumed)
		}
	}
}

func TestUnpackVarint_EmptyInput(t *testing.T) {
	_, _, err := UnpackVarint([]byte{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrVarintTruncated) {
		t.Fatalf("expected error %v, got %v", ErrVarintTruncated, err)
	}
}

func TestUnpackVarint_Truncated2Byte(t *testing.T) {
	// Encode 64 (needs 2 bytes), then only pass 1 byte.
	buf := PackVarint(64)
	if len(buf) != 2 {
		t.Fatalf("want len %v, got %v", 2, len(buf))
	}
	_, _, err := UnpackVarint(buf[:1])
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrVarintTruncated) {
		t.Fatalf("expected error %v, got %v", ErrVarintTruncated, err)
	}
}

func TestUnpackVarint_Truncated4Byte(t *testing.T) {
	buf := PackVarint(16_384)
	if len(buf) != 4 {
		t.Fatalf("want len %v, got %v", 4, len(buf))
	}
	_, _, err := UnpackVarint(buf[:2])
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrVarintTruncated) {
		t.Fatalf("expected error %v, got %v", ErrVarintTruncated, err)
	}
}

func TestUnpackVarint_Truncated8Byte(t *testing.T) {
	buf := PackVarint(1_073_741_824)
	if len(buf) != 8 {
		t.Fatalf("want len %v, got %v", 8, len(buf))
	}
	_, _, err := UnpackVarint(buf[:4])
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrVarintTruncated) {
		t.Fatalf("expected error %v, got %v", ErrVarintTruncated, err)
	}
}

func TestUnpackVarint_ExtraBytes(t *testing.T) {
	// Ensure that extra trailing bytes are not consumed.
	buf := append(PackVarint(42), 0xFF, 0xFF)
	decoded, consumed, err := UnpackVarint(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint64(42), decoded) {
		t.Fatalf("want %v, got %v", uint64(42), decoded)
	}
	if !reflect.DeepEqual(1, consumed) {
		t.Fatalf("want %v, got %v", 1, consumed)
	}
}

func TestPackVarint_SizeMarkBits(t *testing.T) {
	tests := []struct {
		name     string
		value    uint64
		wantMark byte
	}{
		{"1-byte", 0, 0x00},
		{"2-byte", 64, 0x01},
		{"4-byte", 16_384, 0x02},
		{"8-byte", 1_073_741_824, 0x03},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PackVarint(tc.value)
			if !reflect.DeepEqual(tc.wantMark, got[0]&0x03) {
				t.Fatalf("want %v, got %v", tc.wantMark, got[0]&0x03)
			}
		})
	}
}
