package node

import (
	"crypto/ed25519"

	core "dappco.re/go"
)

var (
	// ErrEnvelopeSignatureInvalid means a signed envelope failed verification.
	ErrEnvelopeSignatureInvalid = core.NewError("invalid envelope signature")
	ErrEnvelopeBodyEmpty        = core.NewError("empty envelope body")
)

// Envelope wraps a message body with optional sender signature metadata.
type Envelope struct {
	PeerPubkey []byte `json:"peerPubkey,omitempty"`
	Body       []byte `json:"body"`
	Signature  []byte `json:"signature,omitempty"`
}

// VerifySignature verifies signed envelopes and accepts unsigned envelopes for
// backward compatibility during the identity migration.
func (e Envelope) VerifySignature() core.Result {
	if len(e.Signature) == 0 {
		return core.Ok(nil)
	}
	if len(e.PeerPubkey) != ed25519.PublicKeySize || len(e.Signature) != ed25519.SignatureSize {
		return core.Fail(ErrEnvelopeSignatureInvalid)
	}
	if !ed25519.Verify(ed25519.PublicKey(e.PeerPubkey), e.Body, e.Signature) {
		return core.Fail(ErrEnvelopeSignatureInvalid)
	}
	return core.Ok(nil)
}

func decodeReceivedMessage(data []byte) core.Result {
	bodyResult := unwrapEnvelope(data)
	if !bodyResult.OK {
		return bodyResult
	}
	body := bodyResult.Value.([]byte)

	r := decodeMessageJSON(body)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return core.Fail(err)
		}
		return core.Fail(core.NewError("message unmarshal failed"))
	}
	return r
}

func unwrapEnvelope(data []byte) core.Result {
	envelope := decodeEnvelope(data)
	if !envelope.OK {
		return envelope
	}
	decoded := envelope.Value.(decodeEnvelopeResult)
	if !decoded.OK {
		return core.Ok(data)
	}
	if len(decoded.Envelope.Body) == 0 {
		return core.Fail(ErrEnvelopeBodyEmpty)
	}
	if verified := decoded.Envelope.VerifySignature(); !verified.OK {
		return verified
	}
	return core.Ok(decoded.Envelope.Body)
}

type decodeEnvelopeResult struct {
	Envelope Envelope
	OK       bool
}

func decodeEnvelope(data []byte) core.Result {
	var probe struct {
		PeerPubkey RawMessage `json:"peerPubkey"`
		Body       RawMessage `json:"body"`
		Signature  RawMessage `json:"signature"`
	}
	r := core.JSONUnmarshal(data, &probe)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return core.Fail(err)
		}
		return core.Fail(core.NewError("envelope probe unmarshal failed"))
	}
	if len(probe.PeerPubkey) == 0 && len(probe.Body) == 0 && len(probe.Signature) == 0 {
		return core.Ok(decodeEnvelopeResult{})
	}

	var wire struct {
		PeerPubkey []byte `json:"peerPubkey,omitempty"`
		Body       []byte `json:"body"`
		Signature  []byte `json:"signature,omitempty"`
	}
	r = core.JSONUnmarshal(data, &wire)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return core.Fail(err)
		}
		return core.Fail(core.NewError("envelope unmarshal failed"))
	}
	return core.Ok(decodeEnvelopeResult{
		Envelope: Envelope{PeerPubkey: wire.PeerPubkey, Body: wire.Body, Signature: wire.Signature},
		OK:       true,
	})
}
