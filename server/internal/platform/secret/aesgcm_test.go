package secret

import (
	"crypto/rand"
	"encoding/base64"
	"slices"
	"strings"
	"testing"
)

func testBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("generate test key")
	}
	box, err := NewBox(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal("create test encryptor")
	}
	return box
}

func TestDecryptRejectsInvalidRecoveryDataWithoutPlaintext(t *testing.T) {
	box := testBox(t)
	plaintext := "synthetic recovery secret"
	ciphertext, nonce, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatal("encrypt test secret")
	}
	if restored, err := box.Decrypt(ciphertext, nonce); err != nil || restored != plaintext {
		t.Fatal("correct key did not restore secret")
	}
	tampered := slices.Clone(ciphertext)
	tampered[0] ^= 1
	for _, test := range []struct {
		name       string
		box        *Box
		ciphertext []byte
		nonce      []byte
	}{
		{name: "wrong key", box: testBox(t), ciphertext: ciphertext, nonce: nonce},
		{name: "missing nonce", box: box, ciphertext: ciphertext},
		{name: "truncated nonce", box: box, ciphertext: ciphertext, nonce: nonce[:len(nonce)-1]},
		{name: "tampered ciphertext", box: box, ciphertext: tampered, nonce: nonce},
	} {
		t.Run(test.name, func(t *testing.T) {
			restored, err := test.box.Decrypt(test.ciphertext, test.nonce)
			if err == nil || restored != "" || strings.Contains(err.Error(), plaintext) {
				t.Fatal("invalid recovery data did not fail without plaintext")
			}
		})
	}
	if box, err := NewBox(""); err == nil || box != nil {
		t.Fatal("missing recovery key was accepted")
	}
}
