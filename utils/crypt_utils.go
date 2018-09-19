package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"golang.org/x/crypto/nacl/box"
	"io"
)

type EC25519PublicKey *[32]byte
type EC25519PrivateKey *[32]byte

func GenerateRandId() *string {
	return GenerateRandIdSized(8)
}

func GenerateRandIdSized(bytes int) *string {
	r := make([]byte, bytes)
	_, err := rand.Read(r)
	if err != nil {
		panic("Failed to get random data")
	}
	val := hex.EncodeToString(r)
	return &val
}

func EncryptMessage(data string, publicKey EC25519PublicKey,
	senderPrivateKey EC25519PrivateKey) string {

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}

	// Seal the message, using the first 24 bytes to store the nonce
	encrypted := box.Seal(nonce[:], []byte(data), &nonce, publicKey, senderPrivateKey)
	return base64.StdEncoding.EncodeToString(encrypted)
}

func DecryptMessage(boxStr string, senderPublicKeyStrBase64 string,
	ourPrivateKey EC25519PrivateKey) (string, bool) {

	senderPublicKeyBytes, err := base64.StdEncoding.DecodeString(senderPublicKeyStrBase64)
	if err != nil {
		return "", false
	}
	var senderPublicKey [32]byte
	copy(senderPublicKey[:], senderPublicKeyBytes)

	boxBytes, err := base64.StdEncoding.DecodeString(boxStr)
	if err != nil {
		return "", false
	}

	var decryptNonce [24]byte
	copy(decryptNonce[:], boxBytes[:24])

	decrypted, ok := box.Open(nil, boxBytes[24:], &decryptNonce, &senderPublicKey, ourPrivateKey)
	if !ok {
		return "", false
	}

	return string(decrypted), true
}
