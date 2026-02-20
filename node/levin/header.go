// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

// Package levin implements the CryptoNote Levin binary protocol.
// It is a standalone package with no imports from the parent node package.
package levin

import (
	"encoding/binary"
	"errors"
)

// HeaderSize is the exact byte length of a serialised Levin header.
const HeaderSize = 33

// Signature is the magic value that opens every Levin packet.
const Signature uint64 = 0x0101010101012101

// MaxPayloadSize is the upper bound we accept for a single payload (100 MB).
const MaxPayloadSize uint64 = 100 * 1024 * 1024

// Return-code constants carried in every Levin response.
const (
	ReturnOK            int32 = 0
	ReturnErrConnection int32 = -1
	ReturnErrFormat     int32 = -7
	ReturnErrSignature  int32 = -13
)

// Command IDs for the CryptoNote P2P layer.
const (
	CommandHandshake       uint32 = 1001
	CommandTimedSync       uint32 = 1002
	CommandPing            uint32 = 1003
	CommandNewBlock        uint32 = 2001
	CommandNewTransactions uint32 = 2002
	CommandRequestObjects  uint32 = 2003
	CommandResponseObjects uint32 = 2004
	CommandRequestChain    uint32 = 2006
	CommandResponseChain   uint32 = 2007
)

// Sentinel errors returned by DecodeHeader.
var (
	ErrBadSignature  = errors.New("levin: bad signature")
	ErrPayloadTooBig = errors.New("levin: payload exceeds maximum size")
)

// Header is the 33-byte packed header that prefixes every Levin message.
type Header struct {
	Signature       uint64
	PayloadSize     uint64
	ExpectResponse  bool
	Command         uint32
	ReturnCode      int32
	Flags           uint32
	ProtocolVersion uint32
}

// EncodeHeader serialises h into a fixed-size 33-byte array (little-endian).
func EncodeHeader(h *Header) [HeaderSize]byte {
	var buf [HeaderSize]byte
	binary.LittleEndian.PutUint64(buf[0:8], h.Signature)
	binary.LittleEndian.PutUint64(buf[8:16], h.PayloadSize)
	if h.ExpectResponse {
		buf[16] = 0x01
	} else {
		buf[16] = 0x00
	}
	binary.LittleEndian.PutUint32(buf[17:21], h.Command)
	binary.LittleEndian.PutUint32(buf[21:25], uint32(h.ReturnCode))
	binary.LittleEndian.PutUint32(buf[25:29], h.Flags)
	binary.LittleEndian.PutUint32(buf[29:33], h.ProtocolVersion)
	return buf
}

// DecodeHeader deserialises a 33-byte array into a Header, validating
// the magic signature.
func DecodeHeader(buf [HeaderSize]byte) (Header, error) {
	var h Header
	h.Signature = binary.LittleEndian.Uint64(buf[0:8])
	if h.Signature != Signature {
		return Header{}, ErrBadSignature
	}
	h.PayloadSize = binary.LittleEndian.Uint64(buf[8:16])
	if h.PayloadSize > MaxPayloadSize {
		return Header{}, ErrPayloadTooBig
	}
	h.ExpectResponse = buf[16] == 0x01
	h.Command = binary.LittleEndian.Uint32(buf[17:21])
	h.ReturnCode = int32(binary.LittleEndian.Uint32(buf[21:25]))
	h.Flags = binary.LittleEndian.Uint32(buf[25:29])
	h.ProtocolVersion = binary.LittleEndian.Uint32(buf[29:33])
	return h, nil
}
