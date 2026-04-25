package node

import (
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"
)

const (
	sharedSecretSize = 32
	subKeySize       = 32

	keyInfoEncryptV1   = "lthn-p2p-encrypt-v1"
	keyInfoMACV1       = "lthn-p2p-mac-v1"
	keyInfoChallengeV1 = "lthn-p2p-challenge-v1"
)

type transportSubKeys struct {
	encKey []byte
	macKey []byte
	chlKey []byte
}

func deriveSubKeys(sharedSecret []byte) (transportSubKeys, error) {
	if len(sharedSecret) != sharedSecretSize {
		return transportSubKeys{}, fmt.Errorf("shared secret length %d, want %d", len(sharedSecret), sharedSecretSize)
	}

	encKey, err := deriveSubKey(sharedSecret, keyInfoEncryptV1)
	if err != nil {
		return transportSubKeys{}, err
	}
	macKey, err := deriveSubKey(sharedSecret, keyInfoMACV1)
	if err != nil {
		return transportSubKeys{}, err
	}
	chlKey, err := deriveSubKey(sharedSecret, keyInfoChallengeV1)
	if err != nil {
		return transportSubKeys{}, err
	}

	return transportSubKeys{
		encKey: encKey,
		macKey: macKey,
		chlKey: chlKey,
	}, nil
}

func deriveSubKey(sharedSecret []byte, info string) ([]byte, error) {
	key, err := hkdf.Expand(sha256.New, sharedSecret, info, subKeySize)
	if err != nil {
		return nil, fmt.Errorf("derive %s key: %w", info, err)
	}
	return key, nil
}
