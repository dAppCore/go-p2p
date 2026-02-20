// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"errors"
)

// Size-mark bits occupying the two lowest bits of the first byte.
const (
	varintMask     = 0x03
	varintMark1    = 0x00 // 1 byte,  max 63
	varintMark2    = 0x01 // 2 bytes, max 16,383
	varintMark4    = 0x02 // 4 bytes, max 1,073,741,823
	varintMark8    = 0x03 // 8 bytes, max 4,611,686,018,427,387,903
	varintMax1     = 63
	varintMax2     = 16_383
	varintMax4     = 1_073_741_823
	varintMax8     = 4_611_686_018_427_387_903
)

// ErrVarintTruncated is returned when the buffer is too short.
var ErrVarintTruncated = errors.New("levin: truncated varint")

// ErrVarintOverflow is returned when the value is too large to encode.
var ErrVarintOverflow = errors.New("levin: varint overflow")

// PackVarint encodes v using the epee portable-storage varint scheme.
// The low two bits of the first byte indicate the total encoded width;
// the remaining bits carry the value in little-endian order.
func PackVarint(v uint64) []byte {
	switch {
	case v <= varintMax1:
		return []byte{byte((v << 2) | varintMark1)}
	case v <= varintMax2:
		raw := uint16((v << 2) | varintMark2)
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, raw)
		return buf
	case v <= varintMax4:
		raw := uint32((v << 2) | varintMark4)
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, raw)
		return buf
	default:
		raw := (v << 2) | varintMark8
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, raw)
		return buf
	}
}

// UnpackVarint decodes one epee portable-storage varint from buf.
// It returns the decoded value, the number of bytes consumed, and any error.
func UnpackVarint(buf []byte) (value uint64, bytesConsumed int, err error) {
	if len(buf) == 0 {
		return 0, 0, ErrVarintTruncated
	}

	mark := buf[0] & varintMask

	switch mark {
	case varintMark1:
		value = uint64(buf[0]) >> 2
		return value, 1, nil
	case varintMark2:
		if len(buf) < 2 {
			return 0, 0, ErrVarintTruncated
		}
		raw := binary.LittleEndian.Uint16(buf[:2])
		value = uint64(raw) >> 2
		return value, 2, nil
	case varintMark4:
		if len(buf) < 4 {
			return 0, 0, ErrVarintTruncated
		}
		raw := binary.LittleEndian.Uint32(buf[:4])
		value = uint64(raw) >> 2
		return value, 4, nil
	case varintMark8:
		if len(buf) < 8 {
			return 0, 0, ErrVarintTruncated
		}
		raw := binary.LittleEndian.Uint64(buf[:8])
		value = raw >> 2
		return value, 8, nil
	default:
		// Unreachable — mark is masked to 2 bits.
		return 0, 0, ErrVarintTruncated
	}
}
