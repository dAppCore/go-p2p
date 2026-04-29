package ueps

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"
)

const uepsMACKeyInfoV1 = "lthn-p2p-mac-v1"

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
	Version      uint8 // Default 0x09
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

type tlvWriter interface {
	Write([]byte) (int, error)
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

// MarshalAndSign generates the final byte stream using a domain-separated MAC
// key derived from the shared secret.
func (p *PacketBuilder) MarshalAndSign(sharedSecret []byte) core.Result {
	buf := core.NewBuffer()

	// 1. Write Standard Header Tags (0x01 - 0x05)
	// We write these first because they are part of what we sign.
	if r := writeTLV(buf, TagVersion, []byte{p.Header.Version}); !r.OK {
		return r
	}
	if r := writeTLV(buf, TagCurrentLay, []byte{p.Header.CurrentLayer}); !r.OK {
		return r
	}
	if r := writeTLV(buf, TagTargetLay, []byte{p.Header.TargetLayer}); !r.OK {
		return r
	}
	if r := writeTLV(buf, TagIntent, []byte{p.Header.IntentID}); !r.OK {
		return r
	}

	// Threat Score is uint16, needs binary packing
	tsBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(tsBuf, p.Header.ThreatScore)
	if r := writeTLV(buf, TagThreatScore, tsBuf); !r.OK {
		return r
	}

	// 2. Calculate HMAC
	// The signature covers: Existing Header TLVs + The Payload
	// It does NOT cover the HMAC TLV tag itself (obviously)
	macKeyResult := derivePacketMACKey(sharedSecret)
	if !macKeyResult.OK {
		return macKeyResult
	}
	macKey := macKeyResult.Value.([]byte)
	mac := hmac.New(sha256.New, macKey)
	mac.Write(buf.Bytes()) // The headers so far
	mac.Write(p.Payload)   // The data
	signature := mac.Sum(nil)

	// 3. Write HMAC TLV (0x06)
	// Length is 32 bytes for SHA256
	if r := writeTLV(buf, TagHMAC, signature); !r.OK {
		return r
	}

	// 4. Write Payload TLV (0xFF)
	// Fixed: Now uses writeTLV which provides a 2-byte length prefix.
	// This prevents unbounded read DoS and allows multiple packets in a stream.
	if r := writeTLV(buf, TagPayload, p.Payload); !r.OK {
		return r
	}

	return core.Ok(buf.Bytes())
}

func derivePacketMACKey(sharedSecret []byte) core.Result {
	key, err := hkdf.Expand(sha256.New, sharedSecret, uepsMACKeyInfoV1, sha256.Size)
	if err != nil {
		return core.Fail(core.Errorf("derive UEPS MAC key: %w", err))
	}
	return core.Ok(key)
}

// Helper to write a simple TLV.
// Now uses 2-byte big-endian length (uint16) to support up to 64KB payloads.
func writeTLV(w tlvWriter, tag uint8, value []byte) core.Result {
	// Check length constraint (2 byte length = max 65535 bytes)
	if len(value) > 65535 {
		return core.Fail(coreerr.E("ueps.writeTLV", "TLV value too large for 2-byte length header", nil))
	}

	if _, err := w.Write([]byte{tag}); err != nil {
		return core.Fail(err)
	}

	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(value)))
	if _, err := w.Write(lenBuf); err != nil {
		return core.Fail(err)
	}

	if _, err := w.Write(value); err != nil {
		return core.Fail(err)
	}
	return core.Ok(nil)
}
