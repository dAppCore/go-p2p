package node

import (
	"crypto/hkdf"
	"crypto/sha256"

	core "dappco.re/go"
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

func deriveSubKeys(sharedSecret []byte) core.Result {
	if len(sharedSecret) != sharedSecretSize {
		return core.Fail(core.Errorf("shared secret length %d, want %d", len(sharedSecret), sharedSecretSize))
	}

	encKeyResult := deriveSubKey(sharedSecret, keyInfoEncryptV1)
	if !encKeyResult.OK {
		return encKeyResult
	}
	macKeyResult := deriveSubKey(sharedSecret, keyInfoMACV1)
	if !macKeyResult.OK {
		return macKeyResult
	}
	chlKeyResult := deriveSubKey(sharedSecret, keyInfoChallengeV1)
	if !chlKeyResult.OK {
		return chlKeyResult
	}

	return core.Ok(transportSubKeys{
		encKey: encKeyResult.Value.([]byte),
		macKey: macKeyResult.Value.([]byte),
		chlKey: chlKeyResult.Value.([]byte),
	})
}

func deriveSubKey(sharedSecret []byte, info string) core.Result {
	key, err := hkdf.Expand(sha256.New, sharedSecret, info, subKeySize)
	if err != nil {
		return core.Fail(core.Errorf("derive %s key: %w", info, err))
	}
	return core.Ok(key)
}
