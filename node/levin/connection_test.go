// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestConnection_RoundTrip(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	sender := NewConnection(a)
	receiver := NewConnection(b)

	payload := []byte("hello levin")
	cmd := CommandHandshake

	errCh := make(chan error, 1)
	go func() {
		errCh <- sender.WritePacket(cmd, payload, true)
	}()

	h, data, err := receiver.ReadPacket()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(cmd, h.Command) {
		t.Fatalf("want %v, got %v", cmd, h.Command)
	}
	if !(h.ExpectResponse) {
		t.Fatal("expected true")
	}
	if !reflect.DeepEqual(FlagRequest, h.Flags) {
		t.Fatalf("want %v, got %v", FlagRequest, h.Flags)
	}
	if !reflect.DeepEqual(LevinProtocolVersion, h.ProtocolVersion) {
		t.Fatalf("want %v, got %v", LevinProtocolVersion, h.ProtocolVersion)
	}
	if !reflect.DeepEqual(Signature, h.Signature) {
		t.Fatalf("want %v, got %v", Signature, h.Signature)
	}
	if !reflect.DeepEqual(uint64(len(payload)), h.PayloadSize) {
		t.Fatalf("want %v, got %v", uint64(len(payload)), h.PayloadSize)
	}
	if !reflect.DeepEqual(payload, data) {
		t.Fatalf("want %v, got %v", payload, data)
	}
}

func TestConnection_EmptyPayload(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	sender := NewConnection(a)
	receiver := NewConnection(b)

	errCh := make(chan error, 1)
	go func() {
		errCh <- sender.WritePacket(CommandPing, nil, false)
	}()

	h, data, err := receiver.ReadPacket()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(CommandPing, h.Command) {
		t.Fatalf("want %v, got %v", CommandPing, h.Command)
	}
	if h.ExpectResponse {
		t.Fatal("expected false")
	}
	if !reflect.DeepEqual(uint64(0), h.PayloadSize) {
		t.Fatalf("want %v, got %v", uint64(0), h.PayloadSize)
	}
	if data != nil {
		t.Fatalf("expected nil, got %v", data)
	}
}

func TestConnection_Response(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	sender := NewConnection(a)
	receiver := NewConnection(b)

	payload := []byte("response data")
	retCode := ReturnErrFormat

	errCh := make(chan error, 1)
	go func() {
		errCh <- sender.WriteResponse(CommandHandshake, payload, retCode)
	}()

	h, data, err := receiver.ReadPacket()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(CommandHandshake, h.Command) {
		t.Fatalf("want %v, got %v", CommandHandshake, h.Command)
	}
	if h.ExpectResponse {
		t.Fatal("expected false")
	}
	if !reflect.DeepEqual(retCode, h.ReturnCode) {
		t.Fatalf("want %v, got %v", retCode, h.ReturnCode)
	}
	if !reflect.DeepEqual(FlagResponse, h.Flags) {
		t.Fatalf("want %v, got %v", FlagResponse, h.Flags)
	}
	if !reflect.DeepEqual(payload, data) {
		t.Fatalf("want %v, got %v", payload, data)
	}
}

func TestConnection_PayloadTooBig(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	receiver := NewConnection(b)
	receiver.MaxPayloadSize = 10

	// Manually craft a valid header with PayloadSize = 20 (exceeds limit of 10
	// but is under the package-level MaxPayloadSize so DecodeHeader succeeds).
	h := &Header{
		Signature:       Signature,
		PayloadSize:     20,
		ExpectResponse:  false,
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

	_, _, err := receiver.ReadPacket()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPayloadTooBig) {
		t.Fatalf("expected error %v, got %v", ErrPayloadTooBig, err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnection_ReadTimeout(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	receiver := NewConnection(b)
	receiver.ReadTimeout = 50 * time.Millisecond

	// Do not write anything — the reader should time out.
	_, _, err := receiver.ReadPacket()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify it is a timeout error.
	netErr, ok := err.(net.Error)
	if !(ok) {
		t.Fatal("expected true")
	}
	if !(netErr.Timeout()) {
		t.Fatal("expected true")
	}
}

func TestConnection_RemoteAddr(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	conn := NewConnection(a)
	addr := conn.RemoteAddr()
	if len(addr) == 0 {
		t.Fatal("expected non-empty")
	}
}

func TestConnection_Close(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	conn := NewConnection(a)
	if err := conn.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Writing to a closed connection should fail.
	err := conn.WritePacket(CommandPing, nil, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
