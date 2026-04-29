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
func (e Envelope) VerifySignature() error {
	if len(e.Signature) == 0 {
		return nil
	}
	if len(e.PeerPubkey) != ed25519.PublicKeySize || len(e.Signature) != ed25519.SignatureSize {
		return ErrEnvelopeSignatureInvalid
	}
	if !ed25519.Verify(ed25519.PublicKey(e.PeerPubkey), e.Body, e.Signature) {
		return ErrEnvelopeSignatureInvalid
	}
	return nil
}

func decodeReceivedMessage(data []byte) (*Message, error) {
	body, err := unwrapEnvelope(data)
	if err != nil {
		return nil, err
	}

	var msg Message
	r := core.JSONUnmarshal(body, &msg)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return nil, err
		}
		return nil, core.NewError("message unmarshal failed")
	}
	return &msg, nil
}

func unwrapEnvelope(data []byte) ([]byte, error) {
	env, ok, err := decodeEnvelope(data)
	if err != nil {
		return nil, err
	}
	if !ok {
		return data, nil
	}
	if len(env.Body) == 0 {
		return nil, ErrEnvelopeBodyEmpty
	}
	if err := env.VerifySignature(); err != nil {
		return nil, err
	}
	return env.Body, nil
}

func decodeEnvelope(data []byte) (Envelope, bool, error) {
	var probe struct {
		PeerPubkey RawMessage `json:"peerPubkey"`
		Body       RawMessage `json:"body"`
		Signature  RawMessage `json:"signature"`
	}
	r := core.JSONUnmarshal(data, &probe)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return Envelope{}, false, err
		}
		return Envelope{}, false, core.NewError("envelope probe unmarshal failed")
	}
	if len(probe.PeerPubkey) == 0 && len(probe.Body) == 0 && len(probe.Signature) == 0 {
		return Envelope{}, false, nil
	}

	var env Envelope
	r = core.JSONUnmarshal(data, &env)
	if !r.OK {
		if err, ok := r.Value.(error); ok {
			return Envelope{}, true, err
		}
		return Envelope{}, true, core.NewError("envelope unmarshal failed")
	}
	return env, true, nil
}
