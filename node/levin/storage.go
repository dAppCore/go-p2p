// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"slices"

	coreerr "dappco.re/go/core/log"
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

// AsUint64 returns the uint64 value or an error on type mismatch.
func (v Value) AsUint64() (uint64, error) {
	if v.Type != TypeUint64 {
		return 0, ErrStorageTypeMismatch
	}
	return v.uintVal, nil
}

// AsUint32 returns the uint32 value or an error on type mismatch.
func (v Value) AsUint32() (uint32, error) {
	if v.Type != TypeUint32 {
		return 0, ErrStorageTypeMismatch
	}
	return uint32(v.uintVal), nil
}

// AsUint16 returns the uint16 value or an error on type mismatch.
func (v Value) AsUint16() (uint16, error) {
	if v.Type != TypeUint16 {
		return 0, ErrStorageTypeMismatch
	}
	return uint16(v.uintVal), nil
}

// AsUint8 returns the uint8 value or an error on type mismatch.
func (v Value) AsUint8() (uint8, error) {
	if v.Type != TypeUint8 {
		return 0, ErrStorageTypeMismatch
	}
	return uint8(v.uintVal), nil
}

// AsInt64 returns the int64 value or an error on type mismatch.
func (v Value) AsInt64() (int64, error) {
	if v.Type != TypeInt64 {
		return 0, ErrStorageTypeMismatch
	}
	return v.intVal, nil
}

// AsInt32 returns the int32 value or an error on type mismatch.
func (v Value) AsInt32() (int32, error) {
	if v.Type != TypeInt32 {
		return 0, ErrStorageTypeMismatch
	}
	return int32(v.intVal), nil
}

// AsInt16 returns the int16 value or an error on type mismatch.
func (v Value) AsInt16() (int16, error) {
	if v.Type != TypeInt16 {
		return 0, ErrStorageTypeMismatch
	}
	return int16(v.intVal), nil
}

// AsInt8 returns the int8 value or an error on type mismatch.
func (v Value) AsInt8() (int8, error) {
	if v.Type != TypeInt8 {
		return 0, ErrStorageTypeMismatch
	}
	return int8(v.intVal), nil
}

// AsBool returns the bool value or an error on type mismatch.
func (v Value) AsBool() (bool, error) {
	if v.Type != TypeBool {
		return false, ErrStorageTypeMismatch
	}
	return v.boolVal, nil
}

// AsDouble returns the float64 value or an error on type mismatch.
func (v Value) AsDouble() (float64, error) {
	if v.Type != TypeDouble {
		return 0, ErrStorageTypeMismatch
	}
	return v.floatVal, nil
}

// AsString returns the byte-string value or an error on type mismatch.
func (v Value) AsString() ([]byte, error) {
	if v.Type != TypeString {
		return nil, ErrStorageTypeMismatch
	}
	return v.bytesVal, nil
}

// AsSection returns the nested Section or an error on type mismatch.
func (v Value) AsSection() (Section, error) {
	if v.Type != TypeObject {
		return nil, ErrStorageTypeMismatch
	}
	return v.objectVal, nil
}

// ---------------------------------------------------------------------------
// Array accessors
// ---------------------------------------------------------------------------

// AsUint64Array returns the []uint64 array or an error on type mismatch.
func (v Value) AsUint64Array() ([]uint64, error) {
	if v.Type != (ArrayFlag | TypeUint64) {
		return nil, ErrStorageTypeMismatch
	}
	return v.uint64Array, nil
}

// AsUint32Array returns the []uint32 array or an error on type mismatch.
func (v Value) AsUint32Array() ([]uint32, error) {
	if v.Type != (ArrayFlag | TypeUint32) {
		return nil, ErrStorageTypeMismatch
	}
	return v.uint32Array, nil
}

// AsStringArray returns the [][]byte array or an error on type mismatch.
func (v Value) AsStringArray() ([][]byte, error) {
	if v.Type != (ArrayFlag | TypeString) {
		return nil, ErrStorageTypeMismatch
	}
	return v.stringArray, nil
}

// AsSectionArray returns the []Section array or an error on type mismatch.
func (v Value) AsSectionArray() ([]Section, error) {
	if v.Type != (ArrayFlag | TypeObject) {
		return nil, ErrStorageTypeMismatch
	}
	return v.objectArray, nil
}

// ---------------------------------------------------------------------------
// Encoder
// ---------------------------------------------------------------------------

// EncodeStorage serialises a Section to the portable storage binary format,
// including the 9-byte header. Keys are sorted alphabetically to ensure
// deterministic output.
func EncodeStorage(s Section) ([]byte, error) {
	buf := make([]byte, 0, 256)

	// 9-byte storage header.
	var hdr [StorageHeaderSize]byte
	binary.LittleEndian.PutUint32(hdr[0:4], StorageSignatureA)
	binary.LittleEndian.PutUint32(hdr[4:8], StorageSignatureB)
	hdr[8] = StorageVersion
	buf = append(buf, hdr[:]...)

	// Encode root section.
	out, err := encodeSection(buf, s)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// encodeSection appends a section (entry count + entries) to buf.
func encodeSection(buf []byte, s Section) ([]byte, error) {
	// Sort keys for deterministic output.
	keys := slices.Sorted(maps.Keys(s))

	// Entry count as varint.
	buf = append(buf, PackVarint(uint64(len(keys)))...)

	for _, name := range keys {
		v := s[name]

		// Name: uint8 length + raw bytes.
		if len(name) > 255 {
			return nil, ErrStorageNameTooLong
		}
		buf = append(buf, byte(len(name)))
		buf = append(buf, name...)

		// Type tag.
		buf = append(buf, v.Type)

		// Value.
		var err error
		buf, err = encodeValue(buf, v)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}

// encodeValue appends the encoded representation of a value (without the
// type tag, which is written by the caller).
func encodeValue(buf []byte, v Value) ([]byte, error) {
	// Array types.
	if v.Type&ArrayFlag != 0 {
		return encodeArray(buf, v)
	}

	switch v.Type {
	case TypeUint64:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], v.uintVal)
		return append(buf, tmp[:]...), nil

	case TypeInt64:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], uint64(v.intVal))
		return append(buf, tmp[:]...), nil

	case TypeDouble:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(v.floatVal))
		return append(buf, tmp[:]...), nil

	case TypeUint32:
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], uint32(v.uintVal))
		return append(buf, tmp[:]...), nil

	case TypeInt32:
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], uint32(v.intVal))
		return append(buf, tmp[:]...), nil

	case TypeUint16:
		var tmp [2]byte
		binary.LittleEndian.PutUint16(tmp[:], uint16(v.uintVal))
		return append(buf, tmp[:]...), nil

	case TypeInt16:
		var tmp [2]byte
		binary.LittleEndian.PutUint16(tmp[:], uint16(v.intVal))
		return append(buf, tmp[:]...), nil

	case TypeUint8:
		return append(buf, byte(v.uintVal)), nil

	case TypeInt8:
		return append(buf, byte(v.intVal)), nil

	case TypeBool:
		if v.boolVal {
			return append(buf, 1), nil
		}
		return append(buf, 0), nil

	case TypeString:
		buf = append(buf, PackVarint(uint64(len(v.bytesVal)))...)
		return append(buf, v.bytesVal...), nil

	case TypeObject:
		return encodeSection(buf, v.objectVal)

	default:
		return nil, coreerr.E("levin.encodeValue", fmt.Sprintf("unknown type tag: 0x%02x", v.Type), ErrStorageUnknownType)
	}
}

// encodeArray appends array data: varint(count) + packed elements.
func encodeArray(buf []byte, v Value) ([]byte, error) {
	elemType := v.Type & ^ArrayFlag

	switch elemType {
	case TypeUint64:
		buf = append(buf, PackVarint(uint64(len(v.uint64Array)))...)
		for _, n := range v.uint64Array {
			var tmp [8]byte
			binary.LittleEndian.PutUint64(tmp[:], n)
			buf = append(buf, tmp[:]...)
		}
		return buf, nil

	case TypeUint32:
		buf = append(buf, PackVarint(uint64(len(v.uint32Array)))...)
		for _, n := range v.uint32Array {
			var tmp [4]byte
			binary.LittleEndian.PutUint32(tmp[:], n)
			buf = append(buf, tmp[:]...)
		}
		return buf, nil

	case TypeString:
		buf = append(buf, PackVarint(uint64(len(v.stringArray)))...)
		for _, s := range v.stringArray {
			buf = append(buf, PackVarint(uint64(len(s)))...)
			buf = append(buf, s...)
		}
		return buf, nil

	case TypeObject:
		buf = append(buf, PackVarint(uint64(len(v.objectArray)))...)
		var err error
		for _, sec := range v.objectArray {
			buf, err = encodeSection(buf, sec)
			if err != nil {
				return nil, err
			}
		}
		return buf, nil

	default:
		return nil, coreerr.E("levin.encodeArray", fmt.Sprintf("unknown type tag: array of 0x%02x", elemType), ErrStorageUnknownType)
	}
}

// ---------------------------------------------------------------------------
// Decoder
// ---------------------------------------------------------------------------

// DecodeStorage deserialises portable storage binary data (including the
// 9-byte header) into a Section.
func DecodeStorage(data []byte) (Section, error) {
	if len(data) < StorageHeaderSize {
		return nil, ErrStorageTruncated
	}

	sigA := binary.LittleEndian.Uint32(data[0:4])
	sigB := binary.LittleEndian.Uint32(data[4:8])
	ver := data[8]

	if sigA != StorageSignatureA || sigB != StorageSignatureB {
		return nil, ErrStorageBadSignature
	}
	if ver != StorageVersion {
		return nil, ErrStorageBadVersion
	}

	s, _, err := decodeSection(data[StorageHeaderSize:])
	return s, err
}

// decodeSection reads a section from buf and returns the section plus
// the number of bytes consumed.
func decodeSection(buf []byte) (Section, int, error) {
	count, n, err := UnpackVarint(buf)
	if err != nil {
		return nil, 0, coreerr.E("levin.decodeSection", "section entry count", err)
	}
	off := n

	s := make(Section, int(count))

	for range count {
		// Name length (1 byte).
		if off >= len(buf) {
			return nil, 0, ErrStorageTruncated
		}
		nameLen := int(buf[off])
		off++

		// Name bytes.
		if off+nameLen > len(buf) {
			return nil, 0, ErrStorageTruncated
		}
		name := string(buf[off : off+nameLen])
		off += nameLen

		// Type tag (1 byte).
		if off >= len(buf) {
			return nil, 0, ErrStorageTruncated
		}
		tag := buf[off]
		off++

		// Value.
		val, consumed, err := decodeValue(buf[off:], tag)
		if err != nil {
			return nil, 0, coreerr.E("levin.decodeSection", "field "+name, err)
		}
		off += consumed

		s[name] = val
	}

	return s, off, nil
}

// decodeValue reads a value of the given type tag from buf and returns
// the value plus bytes consumed.
func decodeValue(buf []byte, tag uint8) (Value, int, error) {
	// Array types.
	if tag&ArrayFlag != 0 {
		return decodeArray(buf, tag)
	}

	switch tag {
	case TypeUint64:
		if len(buf) < 8 {
			return Value{}, 0, ErrStorageTruncated
		}
		v := binary.LittleEndian.Uint64(buf[:8])
		return Value{Type: TypeUint64, uintVal: v}, 8, nil

	case TypeInt64:
		if len(buf) < 8 {
			return Value{}, 0, ErrStorageTruncated
		}
		v := int64(binary.LittleEndian.Uint64(buf[:8]))
		return Value{Type: TypeInt64, intVal: v}, 8, nil

	case TypeDouble:
		if len(buf) < 8 {
			return Value{}, 0, ErrStorageTruncated
		}
		bits := binary.LittleEndian.Uint64(buf[:8])
		return Value{Type: TypeDouble, floatVal: math.Float64frombits(bits)}, 8, nil

	case TypeUint32:
		if len(buf) < 4 {
			return Value{}, 0, ErrStorageTruncated
		}
		v := binary.LittleEndian.Uint32(buf[:4])
		return Value{Type: TypeUint32, uintVal: uint64(v)}, 4, nil

	case TypeInt32:
		if len(buf) < 4 {
			return Value{}, 0, ErrStorageTruncated
		}
		v := int32(binary.LittleEndian.Uint32(buf[:4]))
		return Value{Type: TypeInt32, intVal: int64(v)}, 4, nil

	case TypeUint16:
		if len(buf) < 2 {
			return Value{}, 0, ErrStorageTruncated
		}
		v := binary.LittleEndian.Uint16(buf[:2])
		return Value{Type: TypeUint16, uintVal: uint64(v)}, 2, nil

	case TypeInt16:
		if len(buf) < 2 {
			return Value{}, 0, ErrStorageTruncated
		}
		v := int16(binary.LittleEndian.Uint16(buf[:2]))
		return Value{Type: TypeInt16, intVal: int64(v)}, 2, nil

	case TypeUint8:
		if len(buf) < 1 {
			return Value{}, 0, ErrStorageTruncated
		}
		return Value{Type: TypeUint8, uintVal: uint64(buf[0])}, 1, nil

	case TypeInt8:
		if len(buf) < 1 {
			return Value{}, 0, ErrStorageTruncated
		}
		return Value{Type: TypeInt8, intVal: int64(int8(buf[0]))}, 1, nil

	case TypeBool:
		if len(buf) < 1 {
			return Value{}, 0, ErrStorageTruncated
		}
		return Value{Type: TypeBool, boolVal: buf[0] != 0}, 1, nil

	case TypeString:
		strLen, n, err := UnpackVarint(buf)
		if err != nil {
			return Value{}, 0, err
		}
		if uint64(len(buf)-n) < strLen {
			return Value{}, 0, ErrStorageTruncated
		}
		data := make([]byte, strLen)
		copy(data, buf[n:n+int(strLen)])
		return Value{Type: TypeString, bytesVal: data}, n + int(strLen), nil

	case TypeObject:
		sec, consumed, err := decodeSection(buf)
		if err != nil {
			return Value{}, 0, err
		}
		return Value{Type: TypeObject, objectVal: sec}, consumed, nil

	default:
		return Value{}, 0, coreerr.E("levin.decodeValue", fmt.Sprintf("unknown type tag: 0x%02x", tag), ErrStorageUnknownType)
	}
}

// decodeArray reads a typed array from buf (tag has ArrayFlag set).
func decodeArray(buf []byte, tag uint8) (Value, int, error) {
	elemType := tag & ^ArrayFlag

	count, n, err := UnpackVarint(buf)
	if err != nil {
		return Value{}, 0, err
	}
	off := n

	switch elemType {
	case TypeUint64:
		arr := make([]uint64, count)
		for i := range count {
			if off+8 > len(buf) {
				return Value{}, 0, ErrStorageTruncated
			}
			arr[i] = binary.LittleEndian.Uint64(buf[off : off+8])
			off += 8
		}
		return Value{Type: tag, uint64Array: arr}, off, nil

	case TypeUint32:
		arr := make([]uint32, count)
		for i := range count {
			if off+4 > len(buf) {
				return Value{}, 0, ErrStorageTruncated
			}
			arr[i] = binary.LittleEndian.Uint32(buf[off : off+4])
			off += 4
		}
		return Value{Type: tag, uint32Array: arr}, off, nil

	case TypeString:
		arr := make([][]byte, count)
		for i := range count {
			strLen, sn, err := UnpackVarint(buf[off:])
			if err != nil {
				return Value{}, 0, err
			}
			off += sn
			if uint64(len(buf)-off) < strLen {
				return Value{}, 0, ErrStorageTruncated
			}
			data := make([]byte, strLen)
			copy(data, buf[off:off+int(strLen)])
			arr[i] = data
			off += int(strLen)
		}
		return Value{Type: tag, stringArray: arr}, off, nil

	case TypeObject:
		arr := make([]Section, count)
		for i := range count {
			sec, consumed, err := decodeSection(buf[off:])
			if err != nil {
				return Value{}, 0, err
			}
			arr[i] = sec
			off += consumed
		}
		return Value{Type: tag, objectArray: arr}, off, nil

	default:
		return Value{}, 0, coreerr.E("levin.decodeArray", fmt.Sprintf("unknown type tag: array of 0x%02x", elemType), ErrStorageUnknownType)
	}
}
