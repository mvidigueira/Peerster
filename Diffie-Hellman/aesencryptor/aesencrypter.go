package aesencryptor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"log"
)

type AESEncrypter struct {
	key   []byte
	block cipher.Block
	gcm   cipher.AEAD
}

func New(key []byte) *AESEncrypter {
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	return &AESEncrypter{
		key:   key,
		block: block,
		gcm:   aesgcm,
	}
}

func (ae *AESEncrypter) Encrypt(plaintext []byte) []byte {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := ae.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext
}

func (ae *AESEncrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	nonce, cText := ciphertext[:ae.gcm.NonceSize()], ciphertext[ae.gcm.NonceSize():]
	plain, err := ae.gcm.Open(nil, nonce, []byte(cText), nil)
	return plain, err
}
