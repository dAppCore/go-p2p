// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"maps"
	"math"
	"slices"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"
)

// Portable storage signatures and version (9-byte header).
const (
	StorageSignatureA uint32 = 0x01011101
	StorageSignatureB uint32 = 0x01020101
	StorageVersion    uint8  = 1
	StorageHeaderSize        = 9
)

// Type tags for portable storage entries.
const (
	TypeInt64  uint8 = 1
	TypeInt32  uint8 = 2
	TypeInt16  uint8 = 3
	TypeInt8   uint8 = 4
	TypeUint64 uint8 = 5
	TypeUint32 uint8 = 6
	TypeUint16 uint8 = 7
	TypeUint8  uint8 = 8
	TypeDouble uint8 = 9
	TypeString uint8 = 10
	TypeBool   uint8 = 11
	TypeObject uint8 = 12

	ArrayFlag uint8 = 0x80
)

// Sentinel errors for storage encoding and decoding.
var (
	ErrStorageBadSignature = coreerr.E("levin.storage", "bad storage signature", nil)
	ErrStorageTruncated    = coreerr.E("levin.storage", "truncated storage data", nil)
	ErrStorageBadVersion   = coreerr.E("levin.storage", "unsupported storage version", nil)
	ErrStorageNameTooLong  = coreerr.E("levin.storage", "entry name exceeds 255 bytes", nil)
	ErrStorageTypeMismatch = coreerr.E("levin.storage", "value type mismatch", nil)
	ErrStorageUnknownType  = coreerr.E("levin.storage", "unknown type tag", nil)
)

// Section is an ordered map of named values forming a portable storage section.
// Field iteration order is always alphabetical by key for deterministic encoding.
type Section map[string]Value

// Value holds a typed portable storage value. Use the constructor functions
// (Uint64Val, StringVal, ObjectVal, etc.) to create instances.
type Value struct {
	Type uint8

	// Exactly one of these is populated, determined by Type.
	intVal    int64
	uintVal   uint64
	floatVal  float64
	boolVal   bool
	bytesVal  []byte
	objectVal Section

	// Arrays — exactly one populated when Type has ArrayFlag set.
	uint64Array []uint64
	uint32Array []uint32
	stringArray [][]byte
	objectArray []Section
}

type decodedSectionResult struct {
	Section  Section
	Consumed int
}

type decodedValueResult struct {
	Value    Value
	Consumed int
}

// ---------------------------------------------------------------------------
// Scalar constructors
// ---------------------------------------------------------------------------

// Uint64Val creates a Value of TypeUint64.
func Uint64Val(v uint64) Value { return Value{Type: TypeUint64, uintVal: v} }

// Uint32Val creates a Value of TypeUint32.
func Uint32Val(v uint32) Value { return Value{Type: TypeUint32, uintVal: uint64(v)} }

// Uint16Val creates a Value of TypeUint16.
func Uint16Val(v uint16) Value { return Value{Type: TypeUint16, uintVal: uint64(v)} }

// Uint8Val creates a Value of TypeUint8.
func Uint8Val(v uint8) Value { return Value{Type: TypeUint8, uintVal: uint64(v)} }

// Int64Val creates a Value of TypeInt64.
func Int64Val(v int64) Value { return Value{Type: TypeInt64, intVal: v} }

// Int32Val creates a Value of TypeInt32.
func Int32Val(v int32) Value { return Value{Type: TypeInt32, intVal: int64(v)} }

// Int16Val creates a Value of TypeInt16.
func Int16Val(v int16) Value { return Value{Type: TypeInt16, intVal: int64(v)} }

// Int8Val creates a Value of TypeInt8.
func Int8Val(v int8) Value { return Value{Type: TypeInt8, intVal: int64(v)} }

// BoolVal creates a Value of TypeBool.
func BoolVal(v bool) Value { return Value{Type: TypeBool, boolVal: v} }

// DoubleVal creates a Value of TypeDouble.
func DoubleVal(v float64) Value { return Value{Type: TypeDouble, floatVal: v} }

// StringVal creates a Value of TypeString. The slice is not copied.
func StringVal(v []byte) Value { return Value{Type: TypeString, bytesVal: v} }

// ObjectVal creates a Value of TypeObject wrapping a nested Section.
func ObjectVal(s Section) Value { return Value{Type: TypeObject, objectVal: s} }

// ---------------------------------------------------------------------------
// Array constructors
// ---------------------------------------------------------------------------

// Uint64ArrayVal creates a typed array of uint64 values.
func Uint64ArrayVal(vs []uint64) Value {
	return Value{Type: ArrayFlag | TypeUint64, uint64Array: vs}
}

// Uint32ArrayVal creates a typed array of uint32 values.
func Uint32ArrayVal(vs []uint32) Value {
	return Value{Type: ArrayFlag | TypeUint32, uint32Array: vs}
}

// StringArrayVal creates a typed array of byte-string values.
func StringArrayVal(vs [][]byte) Value {
	return Value{Type: ArrayFlag | TypeString, stringArray: vs}
}

// ObjectArrayVal creates a typed array of Section values.
func ObjectArrayVal(vs []Section) Value {
	return Value{Type: ArrayFlag | TypeObject, objectArray: vs}
}

// ---------------------------------------------------------------------------
// Scalar accessors
// ---------------------------------------------------------------------------

// AsUint64 returns the uint64 value or a failed Result on type mismatch.
func (v Value) AsUint64() core.Result {
	if v.Type != TypeUint64 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.uintVal)
}

// AsUint32 returns the uint32 value or a failed Result on type mismatch.
func (v Value) AsUint32() core.Result {
	if v.Type != TypeUint32 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(uint32(v.uintVal))
}

// AsUint16 returns the uint16 value or a failed Result on type mismatch.
func (v Value) AsUint16() core.Result {
	if v.Type != TypeUint16 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(uint16(v.uintVal))
}

// AsUint8 returns the uint8 value or a failed Result on type mismatch.
func (v Value) AsUint8() core.Result {
	if v.Type != TypeUint8 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(uint8(v.uintVal))
}

// AsInt64 returns the int64 value or a failed Result on type mismatch.
func (v Value) AsInt64() core.Result {
	if v.Type != TypeInt64 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.intVal)
}

// AsInt32 returns the int32 value or a failed Result on type mismatch.
func (v Value) AsInt32() core.Result {
	if v.Type != TypeInt32 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(int32(v.intVal))
}

// AsInt16 returns the int16 value or a failed Result on type mismatch.
func (v Value) AsInt16() core.Result {
	if v.Type != TypeInt16 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(int16(v.intVal))
}

// AsInt8 returns the int8 value or a failed Result on type mismatch.
func (v Value) AsInt8() core.Result {
	if v.Type != TypeInt8 {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(int8(v.intVal))
}

// AsBool returns the bool value or a failed Result on type mismatch.
func (v Value) AsBool() core.Result {
	if v.Type != TypeBool {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.boolVal)
}

// AsDouble returns the float64 value or a failed Result on type mismatch.
func (v Value) AsDouble() core.Result {
	if v.Type != TypeDouble {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.floatVal)
}

// AsString returns the byte-string value or a failed Result on type mismatch.
func (v Value) AsString() core.Result {
	if v.Type != TypeString {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.bytesVal)
}

// AsSection returns the nested Section or a failed Result on type mismatch.
func (v Value) AsSection() core.Result {
	if v.Type != TypeObject {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.objectVal)
}

// ---------------------------------------------------------------------------
// Array accessors
// ---------------------------------------------------------------------------

// AsUint64Array returns the []uint64 array or a failed Result on type mismatch.
func (v Value) AsUint64Array() core.Result {
	if v.Type != (ArrayFlag | TypeUint64) {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.uint64Array)
}

// AsUint32Array returns the []uint32 array or a failed Result on type mismatch.
func (v Value) AsUint32Array() core.Result {
	if v.Type != (ArrayFlag | TypeUint32) {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.uint32Array)
}

// AsStringArray returns the [][]byte array or a failed Result on type mismatch.
func (v Value) AsStringArray() core.Result {
	if v.Type != (ArrayFlag | TypeString) {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.stringArray)
}

// AsSectionArray returns the []Section array or a failed Result on type mismatch.
func (v Value) AsSectionArray() core.Result {
	if v.Type != (ArrayFlag | TypeObject) {
		return core.Fail(ErrStorageTypeMismatch)
	}
	return core.Ok(v.objectArray)
}

// ---------------------------------------------------------------------------
// Encoder
// ---------------------------------------------------------------------------

// EncodeStorage serialises a Section to the portable storage binary format,
// including the 9-byte header. Keys are sorted alphabetically to ensure
// deterministic output.
func EncodeStorage(s Section) core.Result {
	buf := make([]byte, 0, 256)

	// 9-byte storage header.
	var hdr [StorageHeaderSize]byte
	binary.LittleEndian.PutUint32(hdr[0:4], StorageSignatureA)
	binary.LittleEndian.PutUint32(hdr[4:8], StorageSignatureB)
	hdr[8] = StorageVersion
	buf = append(buf, hdr[:]...)

	// Encode root section.
	out := encodeSection(buf, s)
	if !out.OK {
		return out
	}
	return out
}

// encodeSection appends a section (entry count + entries) to buf.
func encodeSection(buf []byte, s Section) core.Result {
	// Sort keys for deterministic output.
	keys := slices.Sorted(maps.Keys(s))

	// Entry count as varint.
	buf = append(buf, PackVarint(uint64(len(keys)))...)

	for _, name := range keys {
		v := s[name]

		// Name: uint8 length + raw bytes.
		if len(name) > 255 {
			return core.Fail(ErrStorageNameTooLong)
		}
		buf = append(buf, byte(len(name)))
		buf = append(buf, name...)

		// Type tag.
		buf = append(buf, v.Type)

		// Value.
		encoded := encodeValue(buf, v)
		if !encoded.OK {
			return encoded
		}
		buf = encoded.Value.([]byte)
	}

	return core.Ok(buf)
}

// encodeValue appends the encoded representation of a value (without the
// type tag, which is written by the caller).
func encodeValue(buf []byte, v Value) core.Result {
	// Array types.
	if v.Type&ArrayFlag != 0 {
		return encodeArray(buf, v)
	}

	switch v.Type {
	case TypeUint64:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], v.uintVal)
		return core.Ok(append(buf, tmp[:]...))

	case TypeInt64:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], uint64(v.intVal))
		return core.Ok(append(buf, tmp[:]...))

	case TypeDouble:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(v.floatVal))
		return core.Ok(append(buf, tmp[:]...))

	case TypeUint32:
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], uint32(v.uintVal))
		return core.Ok(append(buf, tmp[:]...))

	case TypeInt32:
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], uint32(v.intVal))
		return core.Ok(append(buf, tmp[:]...))

	case TypeUint16:
		var tmp [2]byte
		binary.LittleEndian.PutUint16(tmp[:], uint16(v.uintVal))
		return core.Ok(append(buf, tmp[:]...))

	case TypeInt16:
		var tmp [2]byte
		binary.LittleEndian.PutUint16(tmp[:], uint16(v.intVal))
		return core.Ok(append(buf, tmp[:]...))

	case TypeUint8:
		return core.Ok(append(buf, byte(v.uintVal)))

	case TypeInt8:
		return core.Ok(append(buf, byte(v.intVal)))

	case TypeBool:
		if v.boolVal {
			return core.Ok(append(buf, 1))
		}
		return core.Ok(append(buf, 0))

	case TypeString:
		buf = append(buf, PackVarint(uint64(len(v.bytesVal)))...)
		return core.Ok(append(buf, v.bytesVal...))

	case TypeObject:
		return encodeSection(buf, v.objectVal)

	default:
		return core.Fail(coreerr.E("levin.encodeValue", core.Sprintf("unknown type tag: 0x%02x", v.Type), ErrStorageUnknownType))
	}
}

// encodeArray appends array data: varint(count) + packed elements.
func encodeArray(buf []byte, v Value) core.Result {
	elemType := v.Type & ^ArrayFlag

	switch elemType {
	case TypeUint64:
		buf = append(buf, PackVarint(uint64(len(v.uint64Array)))...)
		for _, n := range v.uint64Array {
			var tmp [8]byte
			binary.LittleEndian.PutUint64(tmp[:], n)
			buf = append(buf, tmp[:]...)
		}
		return core.Ok(buf)

	case TypeUint32:
		buf = append(buf, PackVarint(uint64(len(v.uint32Array)))...)
		for _, n := range v.uint32Array {
			var tmp [4]byte
			binary.LittleEndian.PutUint32(tmp[:], n)
			buf = append(buf, tmp[:]...)
		}
		return core.Ok(buf)

	case TypeString:
		buf = append(buf, PackVarint(uint64(len(v.stringArray)))...)
		for _, s := range v.stringArray {
			buf = append(buf, PackVarint(uint64(len(s)))...)
			buf = append(buf, s...)
		}
		return core.Ok(buf)

	case TypeObject:
		buf = append(buf, PackVarint(uint64(len(v.objectArray)))...)
		for _, sec := range v.objectArray {
			encoded := encodeSection(buf, sec)
			if !encoded.OK {
				return encoded
			}
			buf = encoded.Value.([]byte)
		}
		return core.Ok(buf)

	default:
		return core.Fail(coreerr.E("levin.encodeArray", core.Sprintf("unknown type tag: array of 0x%02x", elemType), ErrStorageUnknownType))
	}
}

// ---------------------------------------------------------------------------
// Decoder
// ---------------------------------------------------------------------------

// DecodeStorage deserialises portable storage binary data (including the
// 9-byte header) into a Section.
func DecodeStorage(data []byte) core.Result {
	if len(data) < StorageHeaderSize {
		return core.Fail(ErrStorageTruncated)
	}

	sigA := binary.LittleEndian.Uint32(data[0:4])
	sigB := binary.LittleEndian.Uint32(data[4:8])
	ver := data[8]

	if sigA != StorageSignatureA || sigB != StorageSignatureB {
		return core.Fail(ErrStorageBadSignature)
	}
	if ver != StorageVersion {
		return core.Fail(ErrStorageBadVersion)
	}

	decoded := decodeSection(data[StorageHeaderSize:])
	if !decoded.OK {
		return decoded
	}
	return core.Ok(decoded.Value.(decodedSectionResult).Section)
}

// decodeSection reads a section from buf and returns the section plus
// the number of bytes consumed.
func decodeSection(buf []byte) core.Result {
	unpacked := UnpackVarint(buf)
	if !unpacked.OK {
		err, _ := unpacked.Value.(error)
		return core.Fail(coreerr.E("levin.decodeSection", "section entry count", err))
	}
	count := unpacked.Value.(unpackVarintResult).Value
	off := unpacked.Value.(unpackVarintResult).BytesConsumed

	s := make(Section, int(count))

	for range count {
		// Name length (1 byte).
		if off >= len(buf) {
			return core.Fail(ErrStorageTruncated)
		}
		nameLen := int(buf[off])
		off++

		// Name bytes.
		if off+nameLen > len(buf) {
			return core.Fail(ErrStorageTruncated)
		}
		name := string(buf[off : off+nameLen])
		off += nameLen

		// Type tag (1 byte).
		if off >= len(buf) {
			return core.Fail(ErrStorageTruncated)
		}
		tag := buf[off]
		off++

		// Value.
		decoded := decodeValue(buf[off:], tag)
		if !decoded.OK {
			err, _ := decoded.Value.(error)
			return core.Fail(coreerr.E("levin.decodeSection", "field "+name, err))
		}
		val := decoded.Value.(decodedValueResult).Value
		consumed := decoded.Value.(decodedValueResult).Consumed
		off += consumed

		s[name] = val
	}

	return core.Ok(decodedSectionResult{Section: s, Consumed: off})
}

// decodeValue reads a value of the given type tag from buf and returns
// the value plus bytes consumed.
func decodeValue(buf []byte, tag uint8) core.Result {
	// Array types.
	if tag&ArrayFlag != 0 {
		return decodeArray(buf, tag)
	}

	switch tag {
	case TypeUint64:
		if len(buf) < 8 {
			return core.Fail(ErrStorageTruncated)
		}
		v := binary.LittleEndian.Uint64(buf[:8])
		return core.Ok(decodedValueResult{Value: Value{Type: TypeUint64, uintVal: v}, Consumed: 8})

	case TypeInt64:
		if len(buf) < 8 {
			return core.Fail(ErrStorageTruncated)
		}
		v := int64(binary.LittleEndian.Uint64(buf[:8]))
		return core.Ok(decodedValueResult{Value: Value{Type: TypeInt64, intVal: v}, Consumed: 8})

	case TypeDouble:
		if len(buf) < 8 {
			return core.Fail(ErrStorageTruncated)
		}
		bits := binary.LittleEndian.Uint64(buf[:8])
		return core.Ok(decodedValueResult{Value: Value{Type: TypeDouble, floatVal: math.Float64frombits(bits)}, Consumed: 8})

	case TypeUint32:
		if len(buf) < 4 {
			return core.Fail(ErrStorageTruncated)
		}
		v := binary.LittleEndian.Uint32(buf[:4])
		return core.Ok(decodedValueResult{Value: Value{Type: TypeUint32, uintVal: uint64(v)}, Consumed: 4})

	case TypeInt32:
		if len(buf) < 4 {
			return core.Fail(ErrStorageTruncated)
		}
		v := int32(binary.LittleEndian.Uint32(buf[:4]))
		return core.Ok(decodedValueResult{Value: Value{Type: TypeInt32, intVal: int64(v)}, Consumed: 4})

	case TypeUint16:
		if len(buf) < 2 {
			return core.Fail(ErrStorageTruncated)
		}
		v := binary.LittleEndian.Uint16(buf[:2])
		return core.Ok(decodedValueResult{Value: Value{Type: TypeUint16, uintVal: uint64(v)}, Consumed: 2})

	case TypeInt16:
		if len(buf) < 2 {
			return core.Fail(ErrStorageTruncated)
		}
		v := int16(binary.LittleEndian.Uint16(buf[:2]))
		return core.Ok(decodedValueResult{Value: Value{Type: TypeInt16, intVal: int64(v)}, Consumed: 2})

	case TypeUint8:
		if len(buf) < 1 {
			return core.Fail(ErrStorageTruncated)
		}
		return core.Ok(decodedValueResult{Value: Value{Type: TypeUint8, uintVal: uint64(buf[0])}, Consumed: 1})

	case TypeInt8:
		if len(buf) < 1 {
			return core.Fail(ErrStorageTruncated)
		}
		return core.Ok(decodedValueResult{Value: Value{Type: TypeInt8, intVal: int64(int8(buf[0]))}, Consumed: 1})

	case TypeBool:
		if len(buf) < 1 {
			return core.Fail(ErrStorageTruncated)
		}
		return core.Ok(decodedValueResult{Value: Value{Type: TypeBool, boolVal: buf[0] != 0}, Consumed: 1})

	case TypeString:
		unpacked := UnpackVarint(buf)
		if !unpacked.OK {
			return unpacked
		}
		strLen := unpacked.Value.(unpackVarintResult).Value
		n := unpacked.Value.(unpackVarintResult).BytesConsumed
		if uint64(len(buf)-n) < strLen {
			return core.Fail(ErrStorageTruncated)
		}
		data := make([]byte, strLen)
		copy(data, buf[n:n+int(strLen)])
		return core.Ok(decodedValueResult{Value: Value{Type: TypeString, bytesVal: data}, Consumed: n + int(strLen)})

	case TypeObject:
		decoded := decodeSection(buf)
		if !decoded.OK {
			return decoded
		}
		sec := decoded.Value.(decodedSectionResult).Section
		consumed := decoded.Value.(decodedSectionResult).Consumed
		return core.Ok(decodedValueResult{Value: Value{Type: TypeObject, objectVal: sec}, Consumed: consumed})

	default:
		return core.Fail(coreerr.E("levin.decodeValue", core.Sprintf("unknown type tag: 0x%02x", tag), ErrStorageUnknownType))
	}
}

// decodeArray reads a typed array from buf (tag has ArrayFlag set).
func decodeArray(buf []byte, tag uint8) core.Result {
	elemType := tag & ^ArrayFlag

	unpacked := UnpackVarint(buf)
	if !unpacked.OK {
		return unpacked
	}
	count := unpacked.Value.(unpackVarintResult).Value
	off := unpacked.Value.(unpackVarintResult).BytesConsumed

	switch elemType {
	case TypeUint64:
		arr := make([]uint64, count)
		for i := range count {
			if off+8 > len(buf) {
				return core.Fail(ErrStorageTruncated)
			}
			arr[i] = binary.LittleEndian.Uint64(buf[off : off+8])
			off += 8
		}
		return core.Ok(decodedValueResult{Value: Value{Type: tag, uint64Array: arr}, Consumed: off})

	case TypeUint32:
		arr := make([]uint32, count)
		for i := range count {
			if off+4 > len(buf) {
				return core.Fail(ErrStorageTruncated)
			}
			arr[i] = binary.LittleEndian.Uint32(buf[off : off+4])
			off += 4
		}
		return core.Ok(decodedValueResult{Value: Value{Type: tag, uint32Array: arr}, Consumed: off})

	case TypeString:
		arr := make([][]byte, count)
		for i := range count {
			unpacked := UnpackVarint(buf[off:])
			if !unpacked.OK {
				return unpacked
			}
			strLen := unpacked.Value.(unpackVarintResult).Value
			sn := unpacked.Value.(unpackVarintResult).BytesConsumed
			off += sn
			if uint64(len(buf)-off) < strLen {
				return core.Fail(ErrStorageTruncated)
			}
			data := make([]byte, strLen)
			copy(data, buf[off:off+int(strLen)])
			arr[i] = data
			off += int(strLen)
		}
		return core.Ok(decodedValueResult{Value: Value{Type: tag, stringArray: arr}, Consumed: off})

	case TypeObject:
		arr := make([]Section, count)
		for i := range count {
			decoded := decodeSection(buf[off:])
			if !decoded.OK {
				return decoded
			}
			sec := decoded.Value.(decodedSectionResult).Section
			consumed := decoded.Value.(decodedSectionResult).Consumed
			arr[i] = sec
			off += consumed
		}
		return core.Ok(decodedValueResult{Value: Value{Type: tag, objectArray: arr}, Consumed: off})

	default:
		return core.Fail(coreerr.E("levin.decodeArray", core.Sprintf("unknown type tag: array of 0x%02x", elemType), ErrStorageUnknownType))
	}
}
