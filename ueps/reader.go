package ueps

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

// ParsedPacket holds the verified data
type ParsedPacket struct {
	Header  UEPSHeader
	Payload []byte
}

// ReadAndVerify reads a UEPS frame from the stream and validates the HMAC.
// It consumes the stream up to the end of the packet.
func ReadAndVerify(r *bufio.Reader, sharedSecret []byte) (*ParsedPacket, error) {
	// Buffer to reconstruct the data for HMAC verification
	var signedData bytes.Buffer
	header := UEPSHeader{}
	var signature []byte
	var payload []byte

	// Loop through TLVs
	for {
		// 1. Read Tag
		tag, err := r.ReadByte()
		if err != nil {
			return nil, err
		}

		// 2. Read Length (2-byte big-endian uint16)
		lenBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, lenBuf); err != nil {
			return nil, err
		}
		length := int(binary.BigEndian.Uint16(lenBuf))

		// 3. Read Value
		value := make([]byte, length)
		if _, err := io.ReadFull(r, value); err != nil {
			return nil, err
		}

		// 4. Handle Tag
		switch tag {
		case TagVersion:
			header.Version = value[0]
			signedData.WriteByte(tag)
			signedData.Write(lenBuf)
			signedData.Write(value)
		case TagCurrentLay:
			header.CurrentLayer = value[0]
			signedData.WriteByte(tag)
			signedData.Write(lenBuf)
			signedData.Write(value)
		case TagTargetLay:
			header.TargetLayer = value[0]
			signedData.WriteByte(tag)
			signedData.Write(lenBuf)
			signedData.Write(value)
		case TagIntent:
			header.IntentID = value[0]
			signedData.WriteByte(tag)
			signedData.Write(lenBuf)
			signedData.Write(value)
		case TagThreatScore:
			header.ThreatScore = binary.BigEndian.Uint16(value)
			signedData.WriteByte(tag)
			signedData.Write(lenBuf)
			signedData.Write(value)
		case TagHMAC:
			signature = value
			// HMAC tag itself is not part of the signed data
		case TagPayload:
			payload = value
			// Exit loop after payload (last tag in UEPS frame)
			// Note: The HMAC covers the Payload but NOT the TagPayload/Length bytes
			// to match the PacketBuilder.MarshalAndSign logic.
			goto verify
		default:
			// Unknown tag (future proofing), verify it but ignore semantics
			signedData.WriteByte(tag)
			signedData.Write(lenBuf)
			signedData.Write(value)
		}
	}

verify:
	if len(signature) == 0 {
		return nil, errors.New("UEPS packet missing HMAC signature")
	}

	// 5. Verify HMAC
	// Reconstruct: Headers (signedData) + Payload
	mac := hmac.New(sha256.New, sharedSecret)
	mac.Write(signedData.Bytes())
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)

	if !hmac.Equal(signature, expectedMAC) {
		return nil, errors.New("integrity violation: HMAC mismatch (ThreatScore +100)")
	}

	return &ParsedPacket{
		Header:  header,
		Payload: payload,
	}, nil
}

