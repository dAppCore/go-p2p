// Copyright (c) 2024-2026 Lethean Contributors
// SPDX-License-Identifier: EUPL-1.2

package levin

import (
	"io"
	"net"
	"sync"
	"time"
)

// Levin protocol flags.
const (
	FlagRequest  uint32 = 0x00000001
	FlagResponse uint32 = 0x00000002
)

// LevinProtocolVersion is the protocol version field written into every header.
const LevinProtocolVersion uint32 = 1

// Default timeout values for Connection read and write operations.
const (
	DefaultReadTimeout  = 120 * time.Second
	DefaultWriteTimeout = 30 * time.Second
)

// Connection wraps a net.Conn and provides framed Levin packet I/O.
// All writes are serialised by an internal mutex, making it safe to call
// WritePacket and WriteResponse concurrently from multiple goroutines.
type Connection struct {
	// MaxPayloadSize is the upper bound accepted for incoming payloads.
	// Defaults to the package-level MaxPayloadSize (100 MB).
	MaxPayloadSize uint64

	// ReadTimeout is the deadline applied before each ReadPacket call.
	ReadTimeout time.Duration

	// WriteTimeout is the deadline applied before each write call.
	WriteTimeout time.Duration

	conn    net.Conn
	writeMu sync.Mutex
}

// NewConnection creates a Connection that wraps conn with sensible defaults.
func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		MaxPayloadSize: MaxPayloadSize,
		ReadTimeout:    DefaultReadTimeout,
		WriteTimeout:   DefaultWriteTimeout,
		conn:           conn,
	}
}

// WritePacket sends a Levin request or notification. It builds a 33-byte
// header, then writes header + payload atomically under the write mutex.
func (c *Connection) WritePacket(cmd uint32, payload []byte, expectResponse bool) error {
	h := Header{
		Signature:       Signature,
		PayloadSize:     uint64(len(payload)),
		ExpectResponse:  expectResponse,
		Command:         cmd,
		ReturnCode:      ReturnOK,
		Flags:           FlagRequest,
		ProtocolVersion: LevinProtocolVersion,
	}
	return c.writeFrame(&h, payload)
}

// WriteResponse sends a Levin response packet with the given return code.
func (c *Connection) WriteResponse(cmd uint32, payload []byte, returnCode int32) error {
	h := Header{
		Signature:       Signature,
		PayloadSize:     uint64(len(payload)),
		ExpectResponse:  false,
		Command:         cmd,
		ReturnCode:      returnCode,
		Flags:           FlagResponse,
		ProtocolVersion: LevinProtocolVersion,
	}
	return c.writeFrame(&h, payload)
}

// writeFrame serialises header + payload and writes them atomically.
func (c *Connection) writeFrame(h *Header, payload []byte) error {
	buf := EncodeHeader(h)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout)); err != nil {
		return err
	}

	if _, err := c.conn.Write(buf[:]); err != nil {
		return err
	}

	if len(payload) > 0 {
		if _, err := c.conn.Write(payload); err != nil {
			return err
		}
	}

	return nil
}

// ReadPacket reads exactly 33 header bytes, validates the signature,
// checks the payload size against MaxPayloadSize, then reads exactly
// PayloadSize bytes of payload data.
func (c *Connection) ReadPacket() (Header, []byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(c.ReadTimeout)); err != nil {
		return Header{}, nil, err
	}

	// Read header.
	var hdrBuf [HeaderSize]byte
	if _, err := io.ReadFull(c.conn, hdrBuf[:]); err != nil {
		return Header{}, nil, err
	}

	h, err := DecodeHeader(hdrBuf)
	if err != nil {
		return Header{}, nil, err
	}

	// Check against the connection-specific payload limit.
	if h.PayloadSize > c.MaxPayloadSize {
		return Header{}, nil, ErrPayloadTooBig
	}

	// Empty payload is valid — return nil data without allocation.
	if h.PayloadSize == 0 {
		return h, nil, nil
	}

	payload := make([]byte, h.PayloadSize)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return Header{}, nil, err
	}

	return h, payload, nil
}

// Close closes the underlying network connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote address of the underlying connection as a string.
func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}
