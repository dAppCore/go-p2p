// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderSizeIs33(t *testing.T) {
	assert.Equal(t, 33, HeaderSize)
}

func TestEncodeHeader_KnownValues(t *testing.T) {
	h := &Header{
		Signature:       Signature,
		PayloadSize:     256,
		ExpectResponse:  true,
		Command:         CommandHandshake,
		ReturnCode:      ReturnOK,
		Flags:           0,
		ProtocolVersion: 0,
	}

	buf := EncodeHeader(h)

	// Verify signature at offset 0.
	sig := binary.LittleEndian.Uint64(buf[0:8])
	assert.Equal(t, Signature, sig)

	// Verify payload size at offset 8.
	ps := binary.LittleEndian.Uint64(buf[8:16])
	assert.Equal(t, uint64(256), ps)

	// Verify expect-response at offset 16.
	assert.Equal(t, byte(0x01), buf[16])

	// Verify command at offset 17.
	cmd := binary.LittleEndian.Uint32(buf[17:21])
	assert.Equal(t, CommandHandshake, cmd)

	// Verify return code at offset 21.
	rc := int32(binary.LittleEndian.Uint32(buf[21:25]))
	assert.Equal(t, ReturnOK, rc)

	// Verify flags at offset 25.
	flags := binary.LittleEndian.Uint32(buf[25:29])
	assert.Equal(t, uint32(0), flags)

	// Verify protocol version at offset 29.
	pv := binary.LittleEndian.Uint32(buf[29:33])
	assert.Equal(t, uint32(0), pv)
}

func TestEncodeHeader_ExpectResponseFalse(t *testing.T) {
	h := &Header{
		Signature:      Signature,
		PayloadSize:    42,
		ExpectResponse: false,
		Command:        CommandPing,
		ReturnCode:     ReturnOK,
	}
	buf := EncodeHeader(h)
	assert.Equal(t, byte(0x00), buf[16])
}

func TestEncodeHeader_NegativeReturnCode(t *testing.T) {
	h := &Header{
		Signature:      Signature,
		PayloadSize:    0,
		ExpectResponse: false,
		Command:        CommandHandshake,
		ReturnCode:     ReturnErrFormat,
	}
	buf := EncodeHeader(h)
	rc := int32(binary.LittleEndian.Uint32(buf[21:25]))
	assert.Equal(t, ReturnErrFormat, rc)
}

func TestDecodeHeader_RoundTrip(t *testing.T) {
	original := &Header{
		Signature:       Signature,
		PayloadSize:     1024,
		ExpectResponse:  true,
		Command:         CommandTimedSync,
		ReturnCode:      ReturnErrConnection,
		Flags:           0,
		ProtocolVersion: 0,
	}

	buf := EncodeHeader(original)
	decoded, err := DecodeHeader(buf)
	require.NoError(t, err)

	assert.Equal(t, original.Signature, decoded.Signature)
	assert.Equal(t, original.PayloadSize, decoded.PayloadSize)
	assert.Equal(t, original.ExpectResponse, decoded.ExpectResponse)
	assert.Equal(t, original.Command, decoded.Command)
	assert.Equal(t, original.ReturnCode, decoded.ReturnCode)
	assert.Equal(t, original.Flags, decoded.Flags)
	assert.Equal(t, original.ProtocolVersion, decoded.ProtocolVersion)
}

func TestDecodeHeader_AllCommands(t *testing.T) {
	commands := []uint32{
		CommandHandshake,
		CommandTimedSync,
		CommandPing,
		CommandNewBlock,
		CommandNewTransactions,
		CommandRequestObjects,
		CommandResponseObjects,
		CommandRequestChain,
		CommandResponseChain,
	}

	for _, cmd := range commands {
		h := &Header{
			Signature:  Signature,
			Command:    cmd,
			ReturnCode: ReturnOK,
		}
		buf := EncodeHeader(h)
		decoded, err := DecodeHeader(buf)
		require.NoError(t, err)
		assert.Equal(t, cmd, decoded.Command)
	}
}

func TestDecodeHeader_BadSignature(t *testing.T) {
	h := &Header{
		Signature:   0xDEADBEEF,
		PayloadSize: 0,
		Command:     CommandPing,
	}
	buf := EncodeHeader(h)
	_, err := DecodeHeader(buf)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBadSignature)
}

func TestDecodeHeader_PayloadTooBig(t *testing.T) {
	h := &Header{
		Signature:   Signature,
		PayloadSize: MaxPayloadSize + 1,
		Command:     CommandHandshake,
	}
	buf := EncodeHeader(h)
	_, err := DecodeHeader(buf)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPayloadTooBig)
}

func TestDecodeHeader_MaxPayloadExact(t *testing.T) {
	h := &Header{
		Signature:   Signature,
		PayloadSize: MaxPayloadSize,
		Command:     CommandHandshake,
	}
	buf := EncodeHeader(h)
	decoded, err := DecodeHeader(buf)
	require.NoError(t, err)
	assert.Equal(t, MaxPayloadSize, decoded.PayloadSize)
}
