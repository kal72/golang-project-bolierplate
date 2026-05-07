package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidPadding    = errors.New("invalid padding")
)

// Encrypt encrypts plaintext using AES-256-CBC.
// The passphrase is hashed with SHA-256 to derive a 32-byte key.
// Output format: base64(iv + ciphertext).
func Encrypt(plaintext, passphrase string) (string, error) {
	key := deriveKey(passphrase)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)

	ciphertext := make([]byte, aes.BlockSize+len(padded))
	iv := ciphertext[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext[aes.BlockSize:], padded)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-CBC ciphertext produced by Encrypt.
func Decrypt(encoded, passphrase string) (string, error) {
	key := deriveKey(passphrase)

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	if len(data) < aes.BlockSize || len(data)%aes.BlockSize != 0 {
		return "", ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv, ct := data[:aes.BlockSize], data[aes.BlockSize:]
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(ct, ct)

	plaintext, err := pkcs7Unpad(ct, aes.BlockSize)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func deriveKey(passphrase string) []byte {
	h := sha256.Sum256([]byte(passphrase))
	return h[:]
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	n := len(data)
	if n == 0 || n%blockSize != 0 {
		return nil, ErrInvalidPadding
	}

	padding := int(data[n-1])
	if padding == 0 || padding > blockSize {
		return nil, ErrInvalidPadding
	}

	for _, b := range data[n-padding:] {
		if int(b) != padding {
			return nil, ErrInvalidPadding
		}
	}

	return data[:n-padding], nil
}
