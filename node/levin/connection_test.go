// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	require.NoError(t, <-errCh)

	assert.Equal(t, cmd, h.Command)
	assert.True(t, h.ExpectResponse)
	assert.Equal(t, FlagRequest, h.Flags)
	assert.Equal(t, LevinProtocolVersion, h.ProtocolVersion)
	assert.Equal(t, Signature, h.Signature)
	assert.Equal(t, uint64(len(payload)), h.PayloadSize)
	assert.Equal(t, payload, data)
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
	require.NoError(t, err)
	require.NoError(t, <-errCh)

	assert.Equal(t, CommandPing, h.Command)
	assert.False(t, h.ExpectResponse)
	assert.Equal(t, uint64(0), h.PayloadSize)
	assert.Nil(t, data)
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
	require.NoError(t, err)
	require.NoError(t, <-errCh)

	assert.Equal(t, CommandHandshake, h.Command)
	assert.False(t, h.ExpectResponse)
	assert.Equal(t, retCode, h.ReturnCode)
	assert.Equal(t, FlagResponse, h.Flags)
	assert.Equal(t, payload, data)
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
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPayloadTooBig)

	require.NoError(t, <-errCh)
}

func TestConnection_ReadTimeout(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	receiver := NewConnection(b)
	receiver.ReadTimeout = 50 * time.Millisecond

	// Do not write anything — the reader should time out.
	_, _, err := receiver.ReadPacket()
	require.Error(t, err)

	// Verify it is a timeout error.
	netErr, ok := err.(net.Error)
	require.True(t, ok, "expected net.Error, got %T", err)
	assert.True(t, netErr.Timeout(), "expected timeout error")
}

func TestConnection_RemoteAddr(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	conn := NewConnection(a)
	addr := conn.RemoteAddr()
	assert.NotEmpty(t, addr)
}

func TestConnection_Close(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()

	conn := NewConnection(a)
	require.NoError(t, conn.Close())

	// Writing to a closed connection should fail.
	err := conn.WritePacket(CommandPing, nil, false)
	require.Error(t, err)
}
