// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"net"
	"testing"

	core "dappco.re/go"
)

// TestConnection_ReadPacket_BadSignature rejects a 33-byte header whose
// signature word does not match the Levin protocol signature.
func TestConnection_ReadPacket_BadSignature(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	receiver := NewConnection(b)

	// A valid-length header with a deliberately wrong signature.
	h := &Header{
		Signature:       Signature ^ 0xFF, // corrupt the signature
		PayloadSize:     0,
		Command:         CommandPing,
		ReturnCode:      ReturnOK,
		Flags:           FlagRequest,
		ProtocolVersion: LevinProtocolVersion,
	}
	hdrBytes := EncodeHeader(h)

	errCh := make(chan error, 1)
	go func() {
		_, err := a.Write(hdrBytes[:])
		errCh <- err
	}()

	_, _, err := levinReadPacket(receiver.ReadPacket())
	if err == nil {
		t.Fatal("expected bad-signature error, got nil")
	}
	if !core.Is(err, ErrBadSignature) {
		t.Fatalf("error: got %v, want ErrBadSignature", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("write header: %v", err)
	}
}

// TestConnection_ReadPacket_TruncatedHeader fails when fewer than HeaderSize
// bytes are available before the stream closes.
func TestConnection_ReadPacket_TruncatedHeader(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	receiver := NewConnection(b)

	go func() {
		// Write a few bytes then close, so io.ReadFull on the header errors.
		_, _ = a.Write([]byte{0x01, 0x02, 0x03})
		_ = a.Close()
	}()

	if _, _, err := levinReadPacket(receiver.ReadPacket()); err == nil {
		t.Fatal("expected truncated-header error, got nil")
	}
}

// TestConnection_ReadPacket_TruncatedPayload fails when the header declares a
// payload larger than the bytes actually delivered before the stream closes.
func TestConnection_ReadPacket_TruncatedPayload(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	receiver := NewConnection(b)

	h := &Header{
		Signature:       Signature,
		PayloadSize:     16, // claim 16 bytes
		Command:         CommandPing,
		ReturnCode:      ReturnOK,
		Flags:           FlagRequest,
		ProtocolVersion: LevinProtocolVersion,
	}
	hdrBytes := EncodeHeader(h)

	go func() {
		_, _ = a.Write(hdrBytes[:])
		_, _ = a.Write([]byte("short")) // only 5 of the 16 declared bytes
		_ = a.Close()
	}()

	if _, _, err := levinReadPacket(receiver.ReadPacket()); err == nil {
		t.Fatal("expected truncated-payload error, got nil")
	}
}
