// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"net"
	"reflect"
	"testing"
	"time"

	core "dappco.re/go"
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
		errCh <- levinResultErr(sender.WritePacket(cmd, payload, true))
	}()

	h, data, err := levinReadPacket(receiver.ReadPacket())
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

func TestConnection_NewConnection_Good(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	conn := NewConnection(a)
	if conn.ReadTimeout != DefaultReadTimeout {
		t.Fatalf("read timeout: got %v", conn.ReadTimeout)
	}
}

func TestConnection_NewConnection_Bad(t *testing.T) {
	conn := NewConnection(nil)
	if conn.conn != nil {
		t.Fatal("expected nil wrapped connection")
	}
	if conn.MaxPayloadSize != MaxPayloadSize {
		t.Fatalf("max payload: got %d", conn.MaxPayloadSize)
	}
}

func TestConnection_NewConnection_Ugly(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	conn := NewConnection(a)
	if conn.WriteTimeout != DefaultWriteTimeout {
		t.Fatalf("write timeout: got %v", conn.WriteTimeout)
	}
}

func TestConnection_Connection_WritePacket_Good(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	conn := NewConnection(a)
	errCh := make(chan error, 1)
	go func() { errCh <- levinResultErr(conn.WritePacket(CommandPing, []byte("ping"), true)) }()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err != nil || <-errCh != nil {
		t.Fatalf("round trip: header=%#v payload=%q err=%v", header, payload, err)
	}
}

func TestConnection_Connection_WritePacket_Bad(t *testing.T) {
	conn := NewConnection(closedPipeConn(t))
	err := levinResultErr(conn.WritePacket(CommandPing, []byte("ping"), true))
	if err == nil {
		t.Fatal("expected closed connection error")
	}
	if conn.conn == nil {
		t.Fatal("wrapped connection should remain set")
	}
}

func TestConnection_Connection_WritePacket_Ugly(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	conn := NewConnection(a)
	errCh := make(chan error, 1)
	go func() { errCh <- levinResultErr(conn.WritePacket(CommandPing, nil, false)) }()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err != nil || <-errCh != nil || header.PayloadSize != 0 || payload != nil {
		t.Fatalf("empty payload round trip failed")
	}
}

func TestConnection_Connection_WriteResponse_Good(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	conn := NewConnection(a)
	errCh := make(chan error, 1)
	go func() { errCh <- levinResultErr(conn.WriteResponse(CommandPing, []byte("pong"), ReturnOK)) }()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err != nil || <-errCh != nil || header.Flags != FlagResponse || string(payload) != "pong" {
		t.Fatalf("response round trip failed")
	}
}

func TestConnection_Connection_WriteResponse_Bad(t *testing.T) {
	conn := NewConnection(closedPipeConn(t))
	err := levinResultErr(conn.WriteResponse(CommandPing, []byte("pong"), ReturnErrConnection))
	if err == nil {
		t.Fatal("expected closed connection error")
	}
	if conn.conn == nil {
		t.Fatal("wrapped connection should remain set")
	}
}

func TestConnection_Connection_WriteResponse_Ugly(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	conn := NewConnection(a)
	errCh := make(chan error, 1)
	go func() { errCh <- levinResultErr(conn.WriteResponse(CommandPing, nil, ReturnErrFormat)) }()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err != nil || <-errCh != nil || header.ReturnCode != ReturnErrFormat || payload != nil {
		t.Fatalf("empty response round trip failed")
	}
}

func TestConnection_Connection_ReadPacket_Good(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go func() { _ = NewConnection(a).WritePacket(CommandPing, []byte("ping"), false) }()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if header.Command != CommandPing || string(payload) != "ping" {
		t.Fatalf("packet: header=%#v payload=%q", header, payload)
	}
}

func TestConnection_Connection_ReadPacket_Bad(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	a.Close()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err == nil {
		t.Fatal("expected read error")
	}
	if header.Signature != 0 || payload != nil {
		t.Fatalf("packet: header=%#v payload=%q", header, payload)
	}
}

func TestConnection_Connection_ReadPacket_Ugly(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go func() {
		encoded := EncodeHeader(&Header{Signature: Signature})
		_, _ = a.Write(encoded[:])
	}()
	header, payload, err := levinReadPacket(NewConnection(b).ReadPacket())
	if err != nil || header.PayloadSize != 0 || payload != nil {
		t.Fatalf("empty packet failed: header=%#v payload=%q err=%v", header, payload, err)
	}
}

func TestConnection_Connection_Close_Good(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	conn := NewConnection(a)
	err := levinResultErr(conn.Close())
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := a.Write([]byte("x")); err == nil {
		t.Fatal("expected write error after close")
	}
}

func TestConnection_Connection_Close_Bad(t *testing.T) {
	conn := NewConnection(closedPipeConn(t))
	err := levinResultErr(conn.Close())
	if err != nil {
		t.Fatalf("Close on closed pipe: %v", err)
	}
	if conn.conn == nil {
		t.Fatal("wrapped connection should remain set")
	}
}

func TestConnection_Connection_Close_Ugly(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	conn := NewConnection(a)
	_ = conn.Close()
	err := levinResultErr(conn.Close())
	if err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestConnection_Connection_RemoteAddr_Good(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	addr := NewConnection(a).RemoteAddr()
	if addr == "" {
		t.Fatal("expected remote address")
	}
}

func TestConnection_Connection_RemoteAddr_Bad(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	addr := NewConnection(a).RemoteAddr()
	if addr != b.LocalAddr().String() {
		t.Fatalf("remote address: got %q want %q", addr, b.LocalAddr().String())
	}
}

func TestConnection_Connection_RemoteAddr_Ugly(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	_ = a.Close()
	addr := NewConnection(a).RemoteAddr()
	if addr == "" {
		t.Fatal("closed pipe should still report remote address")
	}
}

func closedPipeConn(t *testing.T) net.Conn {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { b.Close() })
	if err := a.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	return a
}

func TestConnection_EmptyPayload(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	sender := NewConnection(a)
	receiver := NewConnection(b)

	errCh := make(chan error, 1)
	go func() {
		errCh <- levinResultErr(sender.WritePacket(CommandPing, nil, false))
	}()

	h, data, err := levinReadPacket(receiver.ReadPacket())
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
		errCh <- levinResultErr(sender.WriteResponse(CommandHandshake, payload, retCode))
	}()

	h, data, err := levinReadPacket(receiver.ReadPacket())
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

	_, _, err := levinReadPacket(receiver.ReadPacket())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Is(err, ErrPayloadTooBig) {
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
	_, _, err := levinReadPacket(receiver.ReadPacket())
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
	if err := levinResultErr(conn.Close()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Writing to a closed connection should fail.
	err := levinResultErr(conn.WritePacket(CommandPing, nil, false))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
