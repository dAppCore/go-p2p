package node

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
)

var (
	// ErrEnvelopeSignatureInvalid means a signed envelope failed verification.
	ErrEnvelopeSignatureInvalid = errors.New("invalid envelope signature")
	ErrEnvelopeBodyEmpty        = errors.New("empty envelope body")
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
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
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
		PeerPubkey json.RawMessage `json:"peerPubkey"`
		Body       json.RawMessage `json:"body"`
		Signature  json.RawMessage `json:"signature"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Envelope{}, false, err
	}
	if len(probe.PeerPubkey) == 0 && len(probe.Body) == 0 && len(probe.Signature) == 0 {
		return Envelope{}, false, nil
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, true, err
	}
	return env, true, nil
}
