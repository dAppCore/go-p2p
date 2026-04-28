// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func TestHeaderSizeIs33(t *testing.T) {
	if !reflect.DeepEqual(33, HeaderSize) {
		t.Fatalf("want %v, got %v", 33, HeaderSize)
	}
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
	if !reflect.DeepEqual(Signature, sig) {
		t.Fatalf("want %v, got %v", Signature, sig)
	}

	// Verify payload size at offset 8.
	ps := binary.LittleEndian.Uint64(buf[8:16])
	if !reflect.DeepEqual(uint64(256), ps) {
		t.Fatalf("want %v, got %v", uint64(256), ps)
	}

	// Verify expect-response at offset 16.
	if !reflect.DeepEqual(byte(0x01), buf[16]) {
		t.Fatalf("want %v, got %v", byte(0x01), buf[16])
	}

	// Verify command at offset 17.
	cmd := binary.LittleEndian.Uint32(buf[17:21])
	if !reflect.DeepEqual(CommandHandshake, cmd) {
		t.Fatalf("want %v, got %v", CommandHandshake, cmd)
	}

	// Verify return code at offset 21.
	rc := int32(binary.LittleEndian.Uint32(buf[21:25]))
	if !reflect.DeepEqual(ReturnOK, rc) {
		t.Fatalf("want %v, got %v", ReturnOK, rc)
	}

	// Verify flags at offset 25.
	flags := binary.LittleEndian.Uint32(buf[25:29])
	if !reflect.DeepEqual(uint32(0), flags) {
		t.Fatalf("want %v, got %v", uint32(0), flags)
	}

	// Verify protocol version at offset 29.
	pv := binary.LittleEndian.Uint32(buf[29:33])
	if !reflect.DeepEqual(uint32(0), pv) {
		t.Fatalf("want %v, got %v", uint32(0), pv)
	}
}

func TestHeader_EncodeHeader_Good(t *testing.T) {
	header := &Header{Signature: Signature, PayloadSize: 5, ExpectResponse: true, Command: CommandPing}
	encoded := EncodeHeader(header)
	if binary.LittleEndian.Uint64(encoded[0:8]) != Signature {
		t.Fatal("signature not encoded")
	}
	if encoded[16] != 1 {
		t.Fatalf("expect-response byte: got %d", encoded[16])
	}
}

func TestHeader_EncodeHeader_Bad(t *testing.T) {
	header := &Header{Signature: 0, PayloadSize: 0, ExpectResponse: false}
	encoded := EncodeHeader(header)
	if binary.LittleEndian.Uint64(encoded[0:8]) != 0 {
		t.Fatal("signature should encode as zero")
	}
	if encoded[16] != 0 {
		t.Fatalf("expect-response byte: got %d", encoded[16])
	}
}

func TestHeader_EncodeHeader_Ugly(t *testing.T) {
	header := &Header{Signature: Signature, PayloadSize: MaxPayloadSize, ReturnCode: -1}
	encoded := EncodeHeader(header)
	if len(encoded) != HeaderSize {
		t.Fatalf("header length: got %d", len(encoded))
	}
	if int32(binary.LittleEndian.Uint32(encoded[21:25])) != -1 {
		t.Fatal("return code not encoded")
	}
}

func TestHeader_DecodeHeader_Good(t *testing.T) {
	encoded := EncodeHeader(&Header{Signature: Signature, PayloadSize: 7, Command: CommandPing})
	header, err := DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if header.PayloadSize != 7 || header.Command != CommandPing {
		t.Fatalf("header: got %#v", header)
	}
}

func TestHeader_DecodeHeader_Bad(t *testing.T) {
	encoded := EncodeHeader(&Header{Signature: 0})
	header, err := DecodeHeader(encoded)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("error: got %v", err)
	}
	if header.Signature != 0 {
		t.Fatalf("header: got %#v", header)
	}
}

func TestHeader_DecodeHeader_Ugly(t *testing.T) {
	encoded := EncodeHeader(&Header{Signature: Signature, PayloadSize: MaxPayloadSize + 1})
	header, err := DecodeHeader(encoded)
	if !errors.Is(err, ErrPayloadTooBig) {
		t.Fatalf("error: got %v", err)
	}
	if header.PayloadSize != 0 {
		t.Fatalf("header: got %#v", header)
	}
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
	if !reflect.DeepEqual(byte(0x00), buf[16]) {
		t.Fatalf("want %v, got %v", byte(0x00), buf[16])
	}
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
	if !reflect.DeepEqual(ReturnErrFormat, rc) {
		t.Fatalf("want %v, got %v", ReturnErrFormat, rc)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(original.Signature, decoded.Signature) {
		t.Fatalf("want %v, got %v", original.Signature, decoded.Signature)
	}
	if !reflect.DeepEqual(original.PayloadSize, decoded.PayloadSize) {
		t.Fatalf("want %v, got %v", original.PayloadSize, decoded.PayloadSize)
	}
	if !reflect.DeepEqual(original.ExpectResponse, decoded.ExpectResponse) {
		t.Fatalf("want %v, got %v", original.ExpectResponse, decoded.ExpectResponse)
	}
	if !reflect.DeepEqual(original.Command, decoded.Command) {
		t.Fatalf("want %v, got %v", original.Command, decoded.Command)
	}
	if !reflect.DeepEqual(original.ReturnCode, decoded.ReturnCode) {
		t.Fatalf("want %v, got %v", original.ReturnCode, decoded.ReturnCode)
	}
	if !reflect.DeepEqual(original.Flags, decoded.Flags) {
		t.Fatalf("want %v, got %v", original.Flags, decoded.Flags)
	}
	if !reflect.DeepEqual(original.ProtocolVersion, decoded.ProtocolVersion) {
		t.Fatalf("want %v, got %v", original.ProtocolVersion, decoded.ProtocolVersion)
	}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(cmd, decoded.Command) {
			t.Fatalf("want %v, got %v", cmd, decoded.Command)
		}
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
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("expected error %v, got %v", ErrBadSignature, err)
	}
}

func TestDecodeHeader_PayloadTooBig(t *testing.T) {
	h := &Header{
		Signature:   Signature,
		PayloadSize: MaxPayloadSize + 1,
		Command:     CommandHandshake,
	}
	buf := EncodeHeader(h)
	_, err := DecodeHeader(buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPayloadTooBig) {
		t.Fatalf("expected error %v, got %v", ErrPayloadTooBig, err)
	}
}

func TestDecodeHeader_MaxPayloadExact(t *testing.T) {
	h := &Header{
		Signature:   Signature,
		PayloadSize: MaxPayloadSize,
		Command:     CommandHandshake,
	}
	buf := EncodeHeader(h)
	decoded, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(MaxPayloadSize, decoded.PayloadSize) {
		t.Fatalf("want %v, got %v", MaxPayloadSize, decoded.PayloadSize)
	}
}
