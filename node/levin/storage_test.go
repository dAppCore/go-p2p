// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"errors"
	"reflect"
	"testing"
)

func requireStorageNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireStorageError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
}

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

func TestStorage_EncodeStorage_Good(t *testing.T) {
	data, err := EncodeStorage(Section{"name": StringVal([]byte("alice"))})
	requireStorageNoError(t, err)
	if len(data) <= StorageHeaderSize {
		t.Fatalf("encoded length: got %d", len(data))
	}
}

func TestStorage_EncodeStorage_Bad(t *testing.T) {
	_, err := EncodeStorage(Section{string(make([]byte, 256)): Uint8Val(1)})
	requireStorageError(t, err)
	if !errors.Is(err, ErrStorageNameTooLong) {
		t.Fatalf("error: got %v", err)
	}
}

func TestStorage_EncodeStorage_Ugly(t *testing.T) {
	data, err := EncodeStorage(Section{})
	requireStorageNoError(t, err)
	if len(data) != StorageHeaderSize+1 {
		t.Fatalf("encoded empty length: got %d", len(data))
	}
}

func TestStorage_DecodeStorage_Good(t *testing.T) {
	data, err := EncodeStorage(Section{"answer": Uint64Val(42)})
	requireStorageNoError(t, err)
	section, err := DecodeStorage(data)
	requireStorageNoError(t, err)
	if got, _ := section["answer"].AsUint64(); got != 42 {
		t.Fatalf("answer: got %d", got)
	}
}

func TestStorage_DecodeStorage_Bad(t *testing.T) {
	section, err := DecodeStorage([]byte{0x01, 0x02})
	requireStorageError(t, err)
	if section != nil {
		t.Fatalf("section: got %#v, want nil", section)
	}
}

func TestStorage_DecodeStorage_Ugly(t *testing.T) {
	data, err := EncodeStorage(Section{})
	requireStorageNoError(t, err)
	section, err := DecodeStorage(data)
	requireStorageNoError(t, err)
	if len(section) != 0 {
		t.Fatalf("section length: got %d", len(section))
	}
}

func TestStorage_Uint64Val_Good(t *testing.T) {
	value := Uint64Val(42)
	got, err := value.AsUint64()
	requireStorageNoError(t, err)
	if got != 42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint64Val_Bad(t *testing.T) {
	value := Uint64Val(0)
	got, err := value.AsUint64()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint64Val_Ugly(t *testing.T) {
	value := Uint64Val(^uint64(0))
	got, err := value.AsUint64()
	requireStorageNoError(t, err)
	if got != ^uint64(0) {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint32Val_Good(t *testing.T) {
	value := Uint32Val(42)
	got, err := value.AsUint32()
	requireStorageNoError(t, err)
	if got != 42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint32Val_Bad(t *testing.T) {
	value := Uint32Val(0)
	got, err := value.AsUint32()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint32Val_Ugly(t *testing.T) {
	value := Uint32Val(^uint32(0))
	got, err := value.AsUint32()
	requireStorageNoError(t, err)
	if got != ^uint32(0) {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint16Val_Good(t *testing.T) {
	value := Uint16Val(42)
	got, err := value.AsUint16()
	requireStorageNoError(t, err)
	if got != 42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint16Val_Bad(t *testing.T) {
	value := Uint16Val(0)
	got, err := value.AsUint16()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint16Val_Ugly(t *testing.T) {
	value := Uint16Val(^uint16(0))
	got, err := value.AsUint16()
	requireStorageNoError(t, err)
	if got != ^uint16(0) {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint8Val_Good(t *testing.T) {
	value := Uint8Val(42)
	got, err := value.AsUint8()
	requireStorageNoError(t, err)
	if got != 42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint8Val_Bad(t *testing.T) {
	value := Uint8Val(0)
	got, err := value.AsUint8()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Uint8Val_Ugly(t *testing.T) {
	value := Uint8Val(^uint8(0))
	got, err := value.AsUint8()
	requireStorageNoError(t, err)
	if got != ^uint8(0) {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int64Val_Good(t *testing.T) {
	value := Int64Val(-42)
	got, err := value.AsInt64()
	requireStorageNoError(t, err)
	if got != -42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int64Val_Bad(t *testing.T) {
	value := Int64Val(0)
	got, err := value.AsInt64()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int64Val_Ugly(t *testing.T) {
	value := Int64Val(-1 << 63)
	got, err := value.AsInt64()
	requireStorageNoError(t, err)
	if got != -1<<63 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int32Val_Good(t *testing.T) {
	value := Int32Val(-42)
	got, err := value.AsInt32()
	requireStorageNoError(t, err)
	if got != -42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int32Val_Bad(t *testing.T) {
	value := Int32Val(0)
	got, err := value.AsInt32()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int32Val_Ugly(t *testing.T) {
	value := Int32Val(-1 << 31)
	got, err := value.AsInt32()
	requireStorageNoError(t, err)
	if got != -1<<31 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int16Val_Good(t *testing.T) {
	value := Int16Val(-42)
	got, err := value.AsInt16()
	requireStorageNoError(t, err)
	if got != -42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int16Val_Bad(t *testing.T) {
	value := Int16Val(0)
	got, err := value.AsInt16()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int16Val_Ugly(t *testing.T) {
	value := Int16Val(-1 << 15)
	got, err := value.AsInt16()
	requireStorageNoError(t, err)
	if got != -1<<15 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int8Val_Good(t *testing.T) {
	value := Int8Val(-42)
	got, err := value.AsInt8()
	requireStorageNoError(t, err)
	if got != -42 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int8Val_Bad(t *testing.T) {
	value := Int8Val(0)
	got, err := value.AsInt8()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Int8Val_Ugly(t *testing.T) {
	value := Int8Val(-1 << 7)
	got, err := value.AsInt8()
	requireStorageNoError(t, err)
	if got != -1<<7 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_BoolVal_Good(t *testing.T) {
	value := BoolVal(true)
	got, err := value.AsBool()
	requireStorageNoError(t, err)
	if !got {
		t.Fatal("expected true")
	}
}

func TestStorage_BoolVal_Bad(t *testing.T) {
	value := BoolVal(false)
	got, err := value.AsBool()
	requireStorageNoError(t, err)
	if got {
		t.Fatal("expected false")
	}
}

func TestStorage_BoolVal_Ugly(t *testing.T) {
	value := BoolVal(!BoolVal(false).boolVal)
	got, err := value.AsBool()
	requireStorageNoError(t, err)
	if !got {
		t.Fatal("expected true")
	}
}

func TestStorage_DoubleVal_Good(t *testing.T) {
	value := DoubleVal(1.25)
	got, err := value.AsDouble()
	requireStorageNoError(t, err)
	if got != 1.25 {
		t.Fatalf("value: got %f", got)
	}
}

func TestStorage_DoubleVal_Bad(t *testing.T) {
	value := DoubleVal(0)
	got, err := value.AsDouble()
	requireStorageNoError(t, err)
	if got != 0 {
		t.Fatalf("value: got %f", got)
	}
}

func TestStorage_DoubleVal_Ugly(t *testing.T) {
	value := DoubleVal(-0.5)
	got, err := value.AsDouble()
	requireStorageNoError(t, err)
	if got != -0.5 {
		t.Fatalf("value: got %f", got)
	}
}

func TestStorage_StringVal_Good(t *testing.T) {
	value := StringVal([]byte("hello"))
	got, err := value.AsString()
	requireStorageNoError(t, err)
	if string(got) != "hello" {
		t.Fatalf("value: got %q", got)
	}
}

func TestStorage_StringVal_Bad(t *testing.T) {
	value := StringVal(nil)
	got, err := value.AsString()
	requireStorageNoError(t, err)
	if got != nil {
		t.Fatalf("value: got %q, want nil", got)
	}
}

func TestStorage_StringVal_Ugly(t *testing.T) {
	source := []byte("hello")
	value := StringVal(source)
	source[0] = 'j'
	if got, _ := value.AsString(); string(got) != "jello" {
		t.Fatalf("value should share caller slice")
	}
}

func TestStorage_ObjectVal_Good(t *testing.T) {
	value := ObjectVal(Section{"a": Uint8Val(1)})
	got, err := value.AsSection()
	requireStorageNoError(t, err)
	if _, ok := got["a"]; !ok {
		t.Fatal("expected object field")
	}
}

func TestStorage_ObjectVal_Bad(t *testing.T) {
	value := ObjectVal(nil)
	got, err := value.AsSection()
	requireStorageNoError(t, err)
	if got != nil {
		t.Fatalf("section: got %#v, want nil", got)
	}
}

func TestStorage_ObjectVal_Ugly(t *testing.T) {
	section := Section{"nested": ObjectVal(Section{})}
	value := ObjectVal(section)
	got, err := value.AsSection()
	requireStorageNoError(t, err)
	if !reflect.DeepEqual(section, got) {
		t.Fatalf("section: got %#v", got)
	}
}

func TestStorage_Uint64ArrayVal_Good(t *testing.T) {
	value := Uint64ArrayVal([]uint64{1, 2})
	got, err := value.AsUint64Array()
	requireStorageNoError(t, err)
	if !reflect.DeepEqual([]uint64{1, 2}, got) {
		t.Fatalf("array: got %#v", got)
	}
}

func TestStorage_Uint64ArrayVal_Bad(t *testing.T) {
	value := Uint64ArrayVal(nil)
	got, err := value.AsUint64Array()
	requireStorageNoError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Uint64ArrayVal_Ugly(t *testing.T) {
	source := []uint64{1}
	value := Uint64ArrayVal(source)
	source[0] = 2
	if got, _ := value.AsUint64Array(); got[0] != 2 {
		t.Fatal("array should share caller slice")
	}
}

func TestStorage_Uint32ArrayVal_Good(t *testing.T) {
	value := Uint32ArrayVal([]uint32{1, 2})
	got, err := value.AsUint32Array()
	requireStorageNoError(t, err)
	if !reflect.DeepEqual([]uint32{1, 2}, got) {
		t.Fatalf("array: got %#v", got)
	}
}

func TestStorage_Uint32ArrayVal_Bad(t *testing.T) {
	value := Uint32ArrayVal(nil)
	got, err := value.AsUint32Array()
	requireStorageNoError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Uint32ArrayVal_Ugly(t *testing.T) {
	source := []uint32{1}
	value := Uint32ArrayVal(source)
	source[0] = 2
	if got, _ := value.AsUint32Array(); got[0] != 2 {
		t.Fatal("array should share caller slice")
	}
}

func TestStorage_StringArrayVal_Good(t *testing.T) {
	value := StringArrayVal([][]byte{[]byte("a"), []byte("b")})
	got, err := value.AsStringArray()
	requireStorageNoError(t, err)
	if string(got[1]) != "b" {
		t.Fatalf("array: got %#v", got)
	}
}

func TestStorage_StringArrayVal_Bad(t *testing.T) {
	value := StringArrayVal(nil)
	got, err := value.AsStringArray()
	requireStorageNoError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_StringArrayVal_Ugly(t *testing.T) {
	source := [][]byte{[]byte("a")}
	value := StringArrayVal(source)
	source[0][0] = 'z'
	if got, _ := value.AsStringArray(); string(got[0]) != "z" {
		t.Fatal("array should share caller slice")
	}
}

func TestStorage_ObjectArrayVal_Good(t *testing.T) {
	value := ObjectArrayVal([]Section{{"a": Uint8Val(1)}})
	got, err := value.AsSectionArray()
	requireStorageNoError(t, err)
	if _, ok := got[0]["a"]; !ok {
		t.Fatal("expected object array field")
	}
}

func TestStorage_ObjectArrayVal_Bad(t *testing.T) {
	value := ObjectArrayVal(nil)
	got, err := value.AsSectionArray()
	requireStorageNoError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_ObjectArrayVal_Ugly(t *testing.T) {
	source := []Section{{"a": Uint8Val(1)}}
	value := ObjectArrayVal(source)
	source[0]["b"] = BoolVal(true)
	if got, _ := value.AsSectionArray(); len(got[0]) != 2 {
		t.Fatal("array should share caller section")
	}
}

func TestStorage_Value_AsUint64_Good(t *testing.T) {
	got, err := Uint64Val(9).AsUint64()
	requireStorageNoError(t, err)
	if got != 9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint64_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsUint64()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint64_Ugly(t *testing.T) {
	got, err := Value{}.AsUint64()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint32_Good(t *testing.T) {
	got, err := Uint32Val(9).AsUint32()
	requireStorageNoError(t, err)
	if got != 9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint32_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsUint32()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint32_Ugly(t *testing.T) {
	got, err := Value{}.AsUint32()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint16_Good(t *testing.T) {
	got, err := Uint16Val(9).AsUint16()
	requireStorageNoError(t, err)
	if got != 9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint16_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsUint16()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint16_Ugly(t *testing.T) {
	got, err := Value{}.AsUint16()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint8_Good(t *testing.T) {
	got, err := Uint8Val(9).AsUint8()
	requireStorageNoError(t, err)
	if got != 9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint8_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsUint8()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsUint8_Ugly(t *testing.T) {
	got, err := Value{}.AsUint8()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt64_Good(t *testing.T) {
	got, err := Int64Val(-9).AsInt64()
	requireStorageNoError(t, err)
	if got != -9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt64_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsInt64()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt64_Ugly(t *testing.T) {
	got, err := Value{}.AsInt64()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt32_Good(t *testing.T) {
	got, err := Int32Val(-9).AsInt32()
	requireStorageNoError(t, err)
	if got != -9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt32_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsInt32()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt32_Ugly(t *testing.T) {
	got, err := Value{}.AsInt32()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt16_Good(t *testing.T) {
	got, err := Int16Val(-9).AsInt16()
	requireStorageNoError(t, err)
	if got != -9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt16_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsInt16()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt16_Ugly(t *testing.T) {
	got, err := Value{}.AsInt16()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt8_Good(t *testing.T) {
	got, err := Int8Val(-9).AsInt8()
	requireStorageNoError(t, err)
	if got != -9 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt8_Bad(t *testing.T) {
	got, err := StringVal([]byte("9")).AsInt8()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsInt8_Ugly(t *testing.T) {
	got, err := Value{}.AsInt8()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %d", got)
	}
}

func TestStorage_Value_AsBool_Good(t *testing.T) {
	got, err := BoolVal(true).AsBool()
	requireStorageNoError(t, err)
	if !got {
		t.Fatal("expected true")
	}
}

func TestStorage_Value_AsBool_Bad(t *testing.T) {
	got, err := StringVal([]byte("true")).AsBool()
	requireStorageError(t, err)
	if got {
		t.Fatal("expected false on mismatch")
	}
}

func TestStorage_Value_AsBool_Ugly(t *testing.T) {
	got, err := Value{}.AsBool()
	requireStorageError(t, err)
	if got {
		t.Fatal("expected false on zero value")
	}
}

func TestStorage_Value_AsDouble_Good(t *testing.T) {
	got, err := DoubleVal(2.5).AsDouble()
	requireStorageNoError(t, err)
	if got != 2.5 {
		t.Fatalf("value: got %f", got)
	}
}

func TestStorage_Value_AsDouble_Bad(t *testing.T) {
	got, err := StringVal([]byte("2.5")).AsDouble()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %f", got)
	}
}

func TestStorage_Value_AsDouble_Ugly(t *testing.T) {
	got, err := Value{}.AsDouble()
	requireStorageError(t, err)
	if got != 0 {
		t.Fatalf("value: got %f", got)
	}
}

func TestStorage_Value_AsString_Good(t *testing.T) {
	got, err := StringVal([]byte("agent")).AsString()
	requireStorageNoError(t, err)
	if string(got) != "agent" {
		t.Fatalf("value: got %q", got)
	}
}

func TestStorage_Value_AsString_Bad(t *testing.T) {
	got, err := Uint8Val(1).AsString()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("value: got %q, want nil", got)
	}
}

func TestStorage_Value_AsString_Ugly(t *testing.T) {
	got, err := Value{}.AsString()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("value: got %q, want nil", got)
	}
}

func TestStorage_Value_AsSection_Good(t *testing.T) {
	got, err := ObjectVal(Section{"a": Uint8Val(1)}).AsSection()
	requireStorageNoError(t, err)
	if _, ok := got["a"]; !ok {
		t.Fatal("expected section field")
	}
}

func TestStorage_Value_AsSection_Bad(t *testing.T) {
	got, err := Uint8Val(1).AsSection()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("section: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsSection_Ugly(t *testing.T) {
	got, err := Value{}.AsSection()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("section: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsUint64Array_Good(t *testing.T) {
	got, err := Uint64ArrayVal([]uint64{1}).AsUint64Array()
	requireStorageNoError(t, err)
	if !reflect.DeepEqual([]uint64{1}, got) {
		t.Fatalf("array: got %#v", got)
	}
}

func TestStorage_Value_AsUint64Array_Bad(t *testing.T) {
	got, err := Uint8Val(1).AsUint64Array()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsUint64Array_Ugly(t *testing.T) {
	got, err := Value{}.AsUint64Array()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsUint32Array_Good(t *testing.T) {
	got, err := Uint32ArrayVal([]uint32{1}).AsUint32Array()
	requireStorageNoError(t, err)
	if !reflect.DeepEqual([]uint32{1}, got) {
		t.Fatalf("array: got %#v", got)
	}
}

func TestStorage_Value_AsUint32Array_Bad(t *testing.T) {
	got, err := Uint8Val(1).AsUint32Array()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsUint32Array_Ugly(t *testing.T) {
	got, err := Value{}.AsUint32Array()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsStringArray_Good(t *testing.T) {
	got, err := StringArrayVal([][]byte{[]byte("a")}).AsStringArray()
	requireStorageNoError(t, err)
	if string(got[0]) != "a" {
		t.Fatalf("array: got %#v", got)
	}
}

func TestStorage_Value_AsStringArray_Bad(t *testing.T) {
	got, err := Uint8Val(1).AsStringArray()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsStringArray_Ugly(t *testing.T) {
	got, err := Value{}.AsStringArray()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsSectionArray_Good(t *testing.T) {
	got, err := ObjectArrayVal([]Section{{"a": Uint8Val(1)}}).AsSectionArray()
	requireStorageNoError(t, err)
	if _, ok := got[0]["a"]; !ok {
		t.Fatal("expected section array field")
	}
}

func TestStorage_Value_AsSectionArray_Bad(t *testing.T) {
	got, err := Uint8Val(1).AsSectionArray()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
	}
}

func TestStorage_Value_AsSectionArray_Ugly(t *testing.T) {
	got, err := Value{}.AsSectionArray()
	requireStorageError(t, err)
	if got != nil {
		t.Fatalf("array: got %#v, want nil", got)
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
