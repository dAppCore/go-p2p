package ueps

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"io"

	coreerr "forge.lthn.ai/core/go-log"
)

// TLV Types
const (
	TagVersion     = 0x01
	TagCurrentLay  = 0x02
	TagTargetLay   = 0x03
	TagIntent      = 0x04
	TagThreatScore = 0x05
	TagHMAC        = 0x06 // The Signature
	TagPayload     = 0xFF // The Data
)

// UEPSHeader represents the conscious routing metadata
type UEPSHeader struct {
	Version      uint8  // Default 0x09
	CurrentLayer uint8
	TargetLayer  uint8
	IntentID     uint8  // Semantic Token
	ThreatScore  uint16 // 0-65535
}

// PacketBuilder helps construct a signed UEPS frame
type PacketBuilder struct {
	Header  UEPSHeader
	Payload []byte
}

// NewBuilder creates a packet context for a specific intent
func NewBuilder(intentID uint8, payload []byte) *PacketBuilder {
	return &PacketBuilder{
		Header: UEPSHeader{
			Version:      0x09, // IPv9
			CurrentLayer: 5,    // Application
			TargetLayer:  5,    // Application
			IntentID:     intentID,
			ThreatScore:  0, // Assumed innocent until proven guilty
		},
		Payload: payload,
	}
}

// MarshalAndSign generates the final byte stream using the shared secret
func (p *PacketBuilder) MarshalAndSign(sharedSecret []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	// 1. Write Standard Header Tags (0x01 - 0x05)
	// We write these first because they are part of what we sign.
	if err := writeTLV(buf, TagVersion, []byte{p.Header.Version}); err != nil {
		return nil, err
	}
	if err := writeTLV(buf, TagCurrentLay, []byte{p.Header.CurrentLayer}); err != nil {
		return nil, err
	}
	if err := writeTLV(buf, TagTargetLay, []byte{p.Header.TargetLayer}); err != nil {
		return nil, err
	}
	if err := writeTLV(buf, TagIntent, []byte{p.Header.IntentID}); err != nil {
		return nil, err
	}
	
	// Threat Score is uint16, needs binary packing
	tsBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(tsBuf, p.Header.ThreatScore)
	if err := writeTLV(buf, TagThreatScore, tsBuf); err != nil {
		return nil, err
	}

	// 2. Calculate HMAC
	// The signature covers: Existing Header TLVs + The Payload
	// It does NOT cover the HMAC TLV tag itself (obviously)
	mac := hmac.New(sha256.New, sharedSecret)
	mac.Write(buf.Bytes()) // The headers so far
	mac.Write(p.Payload)   // The data
	signature := mac.Sum(nil)

	// 3. Write HMAC TLV (0x06)
	// Length is 32 bytes for SHA256
	if err := writeTLV(buf, TagHMAC, signature); err != nil {
		return nil, err
	}

	// 4. Write Payload TLV (0xFF)
	// Fixed: Now uses writeTLV which provides a 2-byte length prefix.
	// This prevents the io.ReadAll DoS and allows multiple packets in a stream.
	if err := writeTLV(buf, TagPayload, p.Payload); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Helper to write a simple TLV.
// Now uses 2-byte big-endian length (uint16) to support up to 64KB payloads.
func writeTLV(w io.Writer, tag uint8, value []byte) error {
	// Check length constraint (2 byte length = max 65535 bytes)
	if len(value) > 65535 {
		return coreerr.E("ueps.writeTLV", "TLV value too large for 2-byte length header", nil)
	}

	if _, err := w.Write([]byte{tag}); err != nil {
		return err
	}
	
	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(value)))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	
	if _, err := w.Write(value); err != nil {
		return err
	}
	return nil
}

