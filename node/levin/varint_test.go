// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackVarint_Value5(t *testing.T) {
	// 5 << 2 | 0x00 = 20 = 0x14
	got := PackVarint(5)
	assert.Equal(t, []byte{0x14}, got)
}

func TestPackVarint_Value100(t *testing.T) {
	// 100 << 2 | 0x01 = 401 = 0x0191 → LE [0x91, 0x01]
	got := PackVarint(100)
	assert.Equal(t, []byte{0x91, 0x01}, got)
}

func TestPackVarint_Value65536(t *testing.T) {
	// 65536 << 2 | 0x02 = 262146 = 0x00040002 → LE [0x02, 0x00, 0x04, 0x00]
	got := PackVarint(65536)
	assert.Equal(t, []byte{0x02, 0x00, 0x04, 0x00}, got)
}

func TestPackVarint_Value2Billion(t *testing.T) {
	got := PackVarint(2_000_000_000)
	require.Len(t, got, 8)
	// Low 2 bits must be 0x03 (8-byte mark).
	assert.Equal(t, byte(0x03), got[0]&0x03)
}

func TestPackVarint_Zero(t *testing.T) {
	got := PackVarint(0)
	assert.Equal(t, []byte{0x00}, got)
}

func TestPackVarint_Boundaries(t *testing.T) {
	tests := []struct {
		name     string
		value    uint64
		wantLen  int
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
			assert.Len(t, got, tc.wantLen, "wrong length for value %d", tc.value)
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
		require.NoError(t, err, "value %d", v)
		assert.Equal(t, v, decoded, "mismatch for value %d", v)
		assert.Equal(t, len(buf), consumed, "wrong bytes consumed for value %d", v)
	}
}

func TestUnpackVarint_EmptyInput(t *testing.T) {
	_, _, err := UnpackVarint([]byte{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVarintTruncated)
}

func TestUnpackVarint_Truncated2Byte(t *testing.T) {
	// Encode 64 (needs 2 bytes), then only pass 1 byte.
	buf := PackVarint(64)
	require.Len(t, buf, 2)
	_, _, err := UnpackVarint(buf[:1])
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVarintTruncated)
}

func TestUnpackVarint_Truncated4Byte(t *testing.T) {
	buf := PackVarint(16_384)
	require.Len(t, buf, 4)
	_, _, err := UnpackVarint(buf[:2])
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVarintTruncated)
}

func TestUnpackVarint_Truncated8Byte(t *testing.T) {
	buf := PackVarint(1_073_741_824)
	require.Len(t, buf, 8)
	_, _, err := UnpackVarint(buf[:4])
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVarintTruncated)
}

func TestUnpackVarint_ExtraBytes(t *testing.T) {
	// Ensure that extra trailing bytes are not consumed.
	buf := append(PackVarint(42), 0xFF, 0xFF)
	decoded, consumed, err := UnpackVarint(buf)
	require.NoError(t, err)
	assert.Equal(t, uint64(42), decoded)
	assert.Equal(t, 1, consumed)
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
			assert.Equal(t, tc.wantMark, got[0]&0x03)
		})
	}
}
