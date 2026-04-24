// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"errors"
	"reflect"
	"testing"
)

func TestEncodeStorage_EmptySection(t *testing.T) {
	s := Section{}
	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 9-byte header + 1-byte varint(0) = 10 bytes.
	if len(data) != 10 {
		t.Fatalf("want len %v, got %v", 10, len(data))
	}

	// Verify storage header signatures.
	if !reflect.DeepEqual(byte(0x01), data[0]) {
		t.Fatalf("want %v, got %v", byte(0x01), data[0])
	}
	if !reflect.DeepEqual(byte(0x11), data[1]) {
		t.Fatalf("want %v, got %v", byte(0x11), data[1])
	}
	if !reflect.DeepEqual(byte(0x01), data[2]) {
		t.Fatalf("want %v, got %v", byte(0x01), data[2])
	}
	if !reflect.DeepEqual(byte(0x01), data[3]) {
		t.Fatalf("want %v, got %v", byte(0x01), data[3])
	}
	if !reflect.DeepEqual(byte(0x01), data[4]) {
		t.Fatalf("want %v, got %v", byte(0x01), data[4])
	}
	if !reflect.DeepEqual(byte(0x01), data[5]) {
		t.Fatalf("want %v, got %v", byte(0x01), data[5])
	}
	if !reflect.DeepEqual(byte(0x02), data[6]) {
		t.Fatalf("want %v, got %v", byte(0x02), data[6])
	}
	if !reflect.DeepEqual(byte(0x01), data[7]) {
		t.Fatalf("want %v, got %v", byte(0x01), data[7])
	}

	// Version byte.
	if !reflect.DeepEqual(byte(1), data[8]) {
		t.Fatalf("want %v, got %v", byte(1), data[8])
	}

	// Entry count varint: 0.
	if !reflect.DeepEqual(byte(0x00), data[9]) {
		t.Fatalf("want %v, got %v", byte(0x00), data[9])
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unsigned integers.
	u64, err := decoded["u64"].AsUint64()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint64(0xDEADBEEFCAFEBABE), u64) {
		t.Fatalf("want %v, got %v", uint64(0xDEADBEEFCAFEBABE), u64)
	}

	u32, err := decoded["u32"].AsUint32()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint32(0xCAFEBABE), u32) {
		t.Fatalf("want %v, got %v", uint32(0xCAFEBABE), u32)
	}

	u16, err := decoded["u16"].AsUint16()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint16(0xBEEF), u16) {
		t.Fatalf("want %v, got %v", uint16(0xBEEF), u16)
	}

	u8, err := decoded["u8"].AsUint8()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint8(42), u8) {
		t.Fatalf("want %v, got %v", uint8(42), u8)
	}

	// Signed integers.
	i64, err := decoded["i64"].AsInt64()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(int64(-9223372036854775808), i64) {
		t.Fatalf("want %v, got %v", int64(-9223372036854775808), i64)
	}

	i32, err := decoded["i32"].AsInt32()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(int32(-2147483648), i32) {
		t.Fatalf("want %v, got %v", int32(-2147483648), i32)
	}

	i16, err := decoded["i16"].AsInt16()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(int16(-32768), i16) {
		t.Fatalf("want %v, got %v", int16(-32768), i16)
	}

	i8, err := decoded["i8"].AsInt8()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(int8(-128), i8) {
		t.Fatalf("want %v, got %v", int8(-128), i8)
	}

	// Bool.
	flag, err := decoded["flag"].AsBool()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(flag) {
		t.Fatal("expected true")
	}

	// String.
	str, err := decoded["height"].AsString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]byte("hello world"), str) {
		t.Fatalf("want %v, got %v", []byte("hello world"), str)
	}

	// Double.
	pi, err := decoded["pi"].AsDouble()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(3.141592653589793, pi) {
		t.Fatalf("want %v, got %v", 3.141592653589793, pi)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ver, err := decoded["version"].AsUint32()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint32(1), ver) {
		t.Fatalf("want %v, got %v", uint32(1), ver)
	}

	innerDec, err := decoded["node_data"].AsSection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	port, err := innerDec["port"].AsUint16()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint16(18080), port) {
		t.Fatalf("want %v, got %v", uint16(18080), port)
	}

	host, err := innerDec["host"].AsString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]byte("127.0.0.1"), host) {
		t.Fatalf("want %v, got %v", []byte("127.0.0.1"), host)
	}
}

func TestStorage_Uint64Array(t *testing.T) {
	s := Section{
		"heights": Uint64ArrayVal([]uint64{10, 20, 30}),
	}

	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, err := decoded["heights"].AsUint64Array()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]uint64{10, 20, 30}, arr) {
		t.Fatalf("want %v, got %v", []uint64{10, 20, 30}, arr)
	}
}

func TestStorage_StringArray(t *testing.T) {
	s := Section{
		"peers": StringArrayVal([][]byte{[]byte("foo"), []byte("bar")}),
	}

	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, err := decoded["peers"].AsStringArray()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("want len %v, got %v", 2, len(arr))
	}
	if !reflect.DeepEqual([]byte("foo"), arr[0]) {
		t.Fatalf("want %v, got %v", []byte("foo"), arr[0])
	}
	if !reflect.DeepEqual([]byte("bar"), arr[1]) {
		t.Fatalf("want %v, got %v", []byte("bar"), arr[1])
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, err := decoded["nodes"].AsSectionArray()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("want len %v, got %v", 2, len(arr))
	}

	id1, err := arr[0]["id"].AsUint32()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint32(1), id1) {
		t.Fatalf("want %v, got %v", uint32(1), id1)
	}

	name1, err := arr[0]["name"].AsString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]byte("alice"), name1) {
		t.Fatalf("want %v, got %v", []byte("alice"), name1)
	}

	id2, err := arr[1]["id"].AsUint32()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(uint32(2), id2) {
		t.Fatalf("want %v, got %v", uint32(2), id2)
	}

	name2, err := arr[1]["name"].AsString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]byte("bob"), name2) {
		t.Fatalf("want %v, got %v", []byte("bob"), name2)
	}
}

func TestDecodeStorage_BadSignature(t *testing.T) {
	// Corrupt the first 4 bytes.
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x01, 0x02, 0x01, 0x01, 0x00}
	_, err := DecodeStorage(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrStorageBadSignature) {
		t.Fatalf("expected error %v, got %v", ErrStorageBadSignature, err)
	}
}

func TestDecodeStorage_TooShort(t *testing.T) {
	_, err := DecodeStorage([]byte{0x01, 0x11})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrStorageTruncated) {
		t.Fatalf("expected error %v, got %v", ErrStorageTruncated, err)
	}
}

func TestStorage_ByteIdenticalReencode(t *testing.T) {
	s := Section{
		"alpha":   Uint64Val(999),
		"bravo":   StringVal([]byte("deterministic")),
		"charlie": BoolVal(false),
		"delta": ObjectVal(Section{
			"x": Int32Val(-42),
			"y": Int32Val(100),
		}),
		"echo": Uint64ArrayVal([]uint64{1, 2, 3}),
	}

	data1, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data2, err := EncodeStorage(decoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(data1, data2) {
		t.Fatalf("want %v, got %v", data1, data2)
	}
}

func TestStorage_TypeMismatchErrors(t *testing.T) {
	v := Uint64Val(42)

	_, err := v.AsUint32()
	if !errors.Is(err, ErrStorageTypeMismatch) {
		t.Fatalf("expected error %v, got %v", ErrStorageTypeMismatch, err)
	}

	_, err = v.AsString()
	if !errors.Is(err, ErrStorageTypeMismatch) {
		t.Fatalf("expected error %v, got %v", ErrStorageTypeMismatch, err)
	}

	_, err = v.AsBool()
	if !errors.Is(err, ErrStorageTypeMismatch) {
		t.Fatalf("expected error %v, got %v", ErrStorageTypeMismatch, err)
	}

	_, err = v.AsSection()
	if !errors.Is(err, ErrStorageTypeMismatch) {
		t.Fatalf("expected error %v, got %v", ErrStorageTypeMismatch, err)
	}

	_, err = v.AsUint64Array()
	if !errors.Is(err, ErrStorageTypeMismatch) {
		t.Fatalf("expected error %v, got %v", ErrStorageTypeMismatch, err)
	}
}

func TestStorage_Uint32Array(t *testing.T) {
	s := Section{
		"ports": Uint32ArrayVal([]uint32{8080, 8443, 9090}),
	}

	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, err := decoded["ports"].AsUint32Array()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual([]uint32{8080, 8443, 9090}, arr) {
		t.Fatalf("want %v, got %v", []uint32{8080, 8443, 9090}, arr)
	}
}

func TestDecodeStorage_BadVersion(t *testing.T) {
	// Valid signatures but version 2 instead of 1.
	data := []byte{0x01, 0x11, 0x01, 0x01, 0x01, 0x01, 0x02, 0x01, 0x02, 0x00}
	_, err := DecodeStorage(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrStorageBadVersion) {
		t.Fatalf("expected error %v, got %v", ErrStorageBadVersion, err)
	}
}

func TestStorage_EmptyArrays(t *testing.T) {
	s := Section{
		"empty_u64": Uint64ArrayVal([]uint64{}),
		"empty_str": StringArrayVal([][]byte{}),
		"empty_obj": ObjectArrayVal([]Section{}),
	}

	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u64arr, err := decoded["empty_u64"].AsUint64Array()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(u64arr) != 0 {
		t.Fatalf("expected empty, got %v", u64arr)
	}

	strarr, err := decoded["empty_str"].AsStringArray()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(strarr) != 0 {
		t.Fatalf("expected empty, got %v", strarr)
	}

	objarr, err := decoded["empty_obj"].AsSectionArray()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objarr) != 0 {
		t.Fatalf("expected empty, got %v", objarr)
	}
}

func TestStorage_BoolFalseRoundTrip(t *testing.T) {
	s := Section{
		"off": BoolVal(false),
		"on":  BoolVal(true),
	}

	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	off, err := decoded["off"].AsBool()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if off {
		t.Fatal("expected false")
	}

	on, err := decoded["on"].AsBool()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(on) {
		t.Fatal("expected true")
	}
}
