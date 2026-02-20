// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeStorage_EmptySection(t *testing.T) {
	s := Section{}
	data, err := EncodeStorage(s)
	require.NoError(t, err)

	// 9-byte header + 1-byte varint(0) = 10 bytes.
	assert.Len(t, data, 10)

	// Verify storage header signatures.
	assert.Equal(t, byte(0x01), data[0])
	assert.Equal(t, byte(0x11), data[1])
	assert.Equal(t, byte(0x01), data[2])
	assert.Equal(t, byte(0x01), data[3])
	assert.Equal(t, byte(0x01), data[4])
	assert.Equal(t, byte(0x01), data[5])
	assert.Equal(t, byte(0x02), data[6])
	assert.Equal(t, byte(0x01), data[7])

	// Version byte.
	assert.Equal(t, byte(1), data[8])

	// Entry count varint: 0.
	assert.Equal(t, byte(0x00), data[9])
}

func TestStorage_PrimitivesRoundTrip(t *testing.T) {
	s := Section{
		"u64":    Uint64Val(0xDEADBEEFCAFEBABE),
		"u32":    Uint32Val(0xCAFEBABE),
		"u16":    Uint16Val(0xBEEF),
		"u8":     Uint8Val(42),
		"i64":    Int64Val(-9223372036854775808),
		"i32":    Int32Val(-2147483648),
		"i16":    Int16Val(-32768),
		"i8":     Int8Val(-128),
		"flag":   BoolVal(true),
		"height": StringVal([]byte("hello world")),
		"pi":     DoubleVal(3.141592653589793),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	// Unsigned integers.
	u64, err := decoded["u64"].AsUint64()
	require.NoError(t, err)
	assert.Equal(t, uint64(0xDEADBEEFCAFEBABE), u64)

	u32, err := decoded["u32"].AsUint32()
	require.NoError(t, err)
	assert.Equal(t, uint32(0xCAFEBABE), u32)

	u16, err := decoded["u16"].AsUint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(0xBEEF), u16)

	u8, err := decoded["u8"].AsUint8()
	require.NoError(t, err)
	assert.Equal(t, uint8(42), u8)

	// Signed integers.
	i64, err := decoded["i64"].AsInt64()
	require.NoError(t, err)
	assert.Equal(t, int64(-9223372036854775808), i64)

	i32, err := decoded["i32"].AsInt32()
	require.NoError(t, err)
	assert.Equal(t, int32(-2147483648), i32)

	i16, err := decoded["i16"].AsInt16()
	require.NoError(t, err)
	assert.Equal(t, int16(-32768), i16)

	i8, err := decoded["i8"].AsInt8()
	require.NoError(t, err)
	assert.Equal(t, int8(-128), i8)

	// Bool.
	flag, err := decoded["flag"].AsBool()
	require.NoError(t, err)
	assert.True(t, flag)

	// String.
	str, err := decoded["height"].AsString()
	require.NoError(t, err)
	assert.Equal(t, []byte("hello world"), str)

	// Double.
	pi, err := decoded["pi"].AsDouble()
	require.NoError(t, err)
	assert.Equal(t, 3.141592653589793, pi)
}

func TestStorage_NestedObject(t *testing.T) {
	inner := Section{
		"port": Uint16Val(18080),
		"host": StringVal([]byte("127.0.0.1")),
	}
	outer := Section{
		"node_data": ObjectVal(inner),
		"version":   Uint32Val(1),
	}

	data, err := EncodeStorage(outer)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	ver, err := decoded["version"].AsUint32()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), ver)

	innerDec, err := decoded["node_data"].AsSection()
	require.NoError(t, err)

	port, err := innerDec["port"].AsUint16()
	require.NoError(t, err)
	assert.Equal(t, uint16(18080), port)

	host, err := innerDec["host"].AsString()
	require.NoError(t, err)
	assert.Equal(t, []byte("127.0.0.1"), host)
}

func TestStorage_Uint64Array(t *testing.T) {
	s := Section{
		"heights": Uint64ArrayVal([]uint64{10, 20, 30}),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	arr, err := decoded["heights"].AsUint64Array()
	require.NoError(t, err)
	assert.Equal(t, []uint64{10, 20, 30}, arr)
}

func TestStorage_StringArray(t *testing.T) {
	s := Section{
		"peers": StringArrayVal([][]byte{[]byte("foo"), []byte("bar")}),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	arr, err := decoded["peers"].AsStringArray()
	require.NoError(t, err)
	require.Len(t, arr, 2)
	assert.Equal(t, []byte("foo"), arr[0])
	assert.Equal(t, []byte("bar"), arr[1])
}

func TestStorage_ObjectArray(t *testing.T) {
	sections := []Section{
		{"id": Uint32Val(1), "name": StringVal([]byte("alice"))},
		{"id": Uint32Val(2), "name": StringVal([]byte("bob"))},
	}
	s := Section{
		"nodes": ObjectArrayVal(sections),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	arr, err := decoded["nodes"].AsSectionArray()
	require.NoError(t, err)
	require.Len(t, arr, 2)

	id1, err := arr[0]["id"].AsUint32()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), id1)

	name1, err := arr[0]["name"].AsString()
	require.NoError(t, err)
	assert.Equal(t, []byte("alice"), name1)

	id2, err := arr[1]["id"].AsUint32()
	require.NoError(t, err)
	assert.Equal(t, uint32(2), id2)

	name2, err := arr[1]["name"].AsString()
	require.NoError(t, err)
	assert.Equal(t, []byte("bob"), name2)
}

func TestDecodeStorage_BadSignature(t *testing.T) {
	// Corrupt the first 4 bytes.
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x01, 0x02, 0x01, 0x01, 0x00}
	_, err := DecodeStorage(data)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStorageBadSignature)
}

func TestDecodeStorage_TooShort(t *testing.T) {
	_, err := DecodeStorage([]byte{0x01, 0x11})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStorageTruncated)
}

func TestStorage_ByteIdenticalReencode(t *testing.T) {
	s := Section{
		"alpha":  Uint64Val(999),
		"bravo":  StringVal([]byte("deterministic")),
		"charlie": BoolVal(false),
		"delta": ObjectVal(Section{
			"x": Int32Val(-42),
			"y": Int32Val(100),
		}),
		"echo": Uint64ArrayVal([]uint64{1, 2, 3}),
	}

	data1, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data1)
	require.NoError(t, err)

	data2, err := EncodeStorage(decoded)
	require.NoError(t, err)

	assert.Equal(t, data1, data2, "re-encoded bytes must be identical")
}

func TestStorage_TypeMismatchErrors(t *testing.T) {
	v := Uint64Val(42)

	_, err := v.AsUint32()
	assert.ErrorIs(t, err, ErrStorageTypeMismatch)

	_, err = v.AsString()
	assert.ErrorIs(t, err, ErrStorageTypeMismatch)

	_, err = v.AsBool()
	assert.ErrorIs(t, err, ErrStorageTypeMismatch)

	_, err = v.AsSection()
	assert.ErrorIs(t, err, ErrStorageTypeMismatch)

	_, err = v.AsUint64Array()
	assert.ErrorIs(t, err, ErrStorageTypeMismatch)
}

func TestStorage_Uint32Array(t *testing.T) {
	s := Section{
		"ports": Uint32ArrayVal([]uint32{8080, 8443, 9090}),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	arr, err := decoded["ports"].AsUint32Array()
	require.NoError(t, err)
	assert.Equal(t, []uint32{8080, 8443, 9090}, arr)
}

func TestDecodeStorage_BadVersion(t *testing.T) {
	// Valid signatures but version 2 instead of 1.
	data := []byte{0x01, 0x11, 0x01, 0x01, 0x01, 0x01, 0x02, 0x01, 0x02, 0x00}
	_, err := DecodeStorage(data)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStorageBadVersion)
}

func TestStorage_EmptyArrays(t *testing.T) {
	s := Section{
		"empty_u64":    Uint64ArrayVal([]uint64{}),
		"empty_str":    StringArrayVal([][]byte{}),
		"empty_obj":    ObjectArrayVal([]Section{}),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	u64arr, err := decoded["empty_u64"].AsUint64Array()
	require.NoError(t, err)
	assert.Empty(t, u64arr)

	strarr, err := decoded["empty_str"].AsStringArray()
	require.NoError(t, err)
	assert.Empty(t, strarr)

	objarr, err := decoded["empty_obj"].AsSectionArray()
	require.NoError(t, err)
	assert.Empty(t, objarr)
}

func TestStorage_BoolFalseRoundTrip(t *testing.T) {
	s := Section{
		"off": BoolVal(false),
		"on":  BoolVal(true),
	}

	data, err := EncodeStorage(s)
	require.NoError(t, err)

	decoded, err := DecodeStorage(data)
	require.NoError(t, err)

	off, err := decoded["off"].AsBool()
	require.NoError(t, err)
	assert.False(t, off)

	on, err := decoded["on"].AsBool()
	require.NoError(t, err)
	assert.True(t, on)
}
