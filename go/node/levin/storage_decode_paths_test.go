// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"testing"

	core "dappco.re/go"
)

// encodeForDecode is a small helper that encodes a section and fails the test
// if encoding does not succeed.
func encodeForDecode(t *testing.T, s Section) []byte {
	t.Helper()
	data, err := levinResultValue[[]byte](EncodeStorage(s))
	if err != nil {
		t.Fatalf("EncodeStorage: %v", err)
	}
	return data
}

// TestDecodeStorage_TruncatedScalar truncates a valid uint64-bearing storage one
// byte at a time past the header and asserts every truncation is rejected
// rather than read out of bounds.
func TestDecodeStorage_TruncatedScalar(t *testing.T) {
	data := encodeForDecode(t, Section{"answer": Uint64Val(42)})

	// Every prefix shorter than the full frame (but at least the header) must
	// fail to decode without panicking.
	for cut := StorageHeaderSize; cut < len(data); cut++ {
		if _, err := levinResultValue[Section](DecodeStorage(data[:cut])); err == nil {
			t.Fatalf("expected truncation error at cut=%d", cut)
		}
	}

	// The full frame still decodes.
	section, err := levinResultValue[Section](DecodeStorage(data))
	if err != nil {
		t.Fatalf("full frame decode: %v", err)
	}
	if got, _ := levinResultValue[uint64](section["answer"].AsUint64()); got != 42 {
		t.Fatalf("answer: got %d, want 42", got)
	}
}

// TestDecodeStorage_TruncatedString covers the string-length / string-body
// truncation branch in decodeValue.
func TestDecodeStorage_TruncatedString(t *testing.T) {
	data := encodeForDecode(t, Section{"name": StringVal([]byte("alice-bob-carol"))})

	for cut := StorageHeaderSize; cut < len(data); cut++ {
		if _, err := levinResultValue[Section](DecodeStorage(data[:cut])); err == nil {
			t.Fatalf("expected truncation error at cut=%d", cut)
		}
	}
}

// TestDecodeStorage_TruncatedArray covers the per-element truncation branch in
// decodeArray.
func TestDecodeStorage_TruncatedArray(t *testing.T) {
	data := encodeForDecode(t, Section{"nums": Uint64ArrayVal([]uint64{1, 2, 3, 4})})

	for cut := StorageHeaderSize; cut < len(data); cut++ {
		if _, err := levinResultValue[Section](DecodeStorage(data[:cut])); err == nil {
			t.Fatalf("expected truncation error at cut=%d", cut)
		}
	}

	section, err := levinResultValue[Section](DecodeStorage(data))
	if err != nil {
		t.Fatalf("full array decode: %v", err)
	}
	if section["nums"].Type&ArrayFlag == 0 {
		t.Fatal("expected array-flagged value")
	}
}

// TestDecodeStorage_NestedObject decodes a section nested inside another
// section, covering the TypeObject branch of decodeValue.
func TestDecodeStorage_NestedObject(t *testing.T) {
	inner := Section{"inner": Uint32Val(7)}
	data := encodeForDecode(t, Section{"outer": ObjectVal(inner)})

	section, err := levinResultValue[Section](DecodeStorage(data))
	if err != nil {
		t.Fatalf("nested decode: %v", err)
	}
	obj, err := levinResultValue[Section](section["outer"].AsSection())
	if err != nil {
		t.Fatalf("AsSection: %v", err)
	}
	if got, _ := levinResultValue[uint32](obj["inner"].AsUint32()); got != 7 {
		t.Fatalf("inner: got %d, want 7", got)
	}

	// Truncating a nested object must propagate a decode error.
	for cut := StorageHeaderSize; cut < len(data); cut++ {
		if _, err := levinResultValue[Section](DecodeStorage(data[:cut])); err == nil {
			t.Fatalf("expected truncation error at cut=%d", cut)
		}
	}
}

// validStorageHeader returns the 9-byte storage header from a real encode, so
// hand-built malformed bodies pass signature/version validation and reach the
// section decoder.
func validStorageHeader(t *testing.T) []byte {
	t.Helper()
	data := encodeForDecode(t, Section{})
	return data[:StorageHeaderSize]
}

// TestDecodeStorage_UnknownTypeTag rejects a field whose type tag is not a
// known storage type, covering the decodeValue default branch.
func TestDecodeStorage_UnknownTypeTag(t *testing.T) {
	// Valid header, varint count 1, one field named "x" with an unknown type
	// tag 0x7F and no value bytes.
	frame := append([]byte(nil), validStorageHeader(t)...)
	frame = append(frame, PackVarint(1)...) // entry count varint: 1
	frame = append(frame, 0x01)             // name length (raw byte): 1
	frame = append(frame, 'x')              // name: "x"
	frame = append(frame, 0x7F)             // unknown type tag (no ArrayFlag)

	_, err := levinResultValue[Section](DecodeStorage(frame))
	if err == nil {
		t.Fatal("expected unknown-type error")
	}
	if !core.Contains(err.Error(), "unknown type tag") {
		t.Fatalf("error: got %v, want unknown-type-tag", err)
	}
}

// TestDecodeStorage_UnknownArrayElemType rejects an array whose element type is
// not a known storage type, covering the decodeArray default branch.
func TestDecodeStorage_UnknownArrayElemType(t *testing.T) {
	// Valid header, count 1, name "x", type tag = ArrayFlag | 0x7F, array count 1.
	frame := append([]byte(nil), validStorageHeader(t)...)
	frame = append(frame, PackVarint(1)...) // entry count: 1
	frame = append(frame, 0x01)             // name length (raw byte): 1
	frame = append(frame, 'x')              // name: "x"
	frame = append(frame, ArrayFlag|0x7F)   // array of unknown element type
	frame = append(frame, PackVarint(1)...) // array element count: 1

	_, err := levinResultValue[Section](DecodeStorage(frame))
	if err == nil {
		t.Fatal("expected unknown-array-element-type error")
	}
	if !core.Contains(err.Error(), "unknown type tag") {
		t.Fatalf("error: got %v, want unknown-type-tag", err)
	}
}

// TestDecodeStorage_TruncatedName covers the truncated-name-bytes branch in
// decodeSection: count claims one field with a name longer than the buffer.
func TestDecodeStorage_TruncatedName(t *testing.T) {
	frame := append([]byte(nil), validStorageHeader(t)...)
	frame = append(frame, PackVarint(1)...) // entry count: 1
	frame = append(frame, 0x05)             // name length (raw byte): 5
	frame = append(frame, 'a')              // only one name byte present

	if _, err := levinResultValue[Section](DecodeStorage(frame)); err == nil {
		t.Fatal("expected truncated-name error")
	}
}
