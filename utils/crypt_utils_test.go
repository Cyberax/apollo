package utils

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/magiconair/properties/assert"
	"golang.org/x/crypto/nacl/box"
	"testing"
)

func TestGenerateRandId(t *testing.T) {
	assert.Equal(t, 16, len(*GenerateRandId()))
	assert.Equal(t, 32, len(*GenerateRandIdSized(16)))
}

func TestSecureBox(t *testing.T) {
	cliPublicKey, cliPrivateKey, _ := box.GenerateKey(rand.Reader)
	senderPublicKey, senderPrivateKey, _ := box.GenerateKey(rand.Reader)

	encrypted := EncryptMessage("hello,world", cliPublicKey, senderPrivateKey)
	message, ok := DecryptMessage(encrypted,
		base64.StdEncoding.EncodeToString(senderPublicKey[:]), cliPrivateKey)
	assert.Equal(t, true, ok)
	assert.Equal(t, "hello,world", message)

	// Corrupt the message
	encrypted = "nopenope" + encrypted[8:]
	_, ok = DecryptMessage(encrypted,
		base64.StdEncoding.EncodeToString(senderPublicKey[:]), cliPrivateKey)
	assert.Equal(t, false, ok)
}
