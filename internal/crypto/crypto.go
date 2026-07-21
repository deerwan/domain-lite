package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

// key 由 JWT secret 派生出 32 字节 AES-256 密钥。
func key(secret string) [32]byte {
	return sha256.Sum256([]byte(secret))
}

// Encrypt AES-256-GCM 加密，返回 hex 字符串（含 nonce 前缀）。
func Encrypt(plain, secret string) (string, error) {
	k := key(secret)
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return hex.EncodeToString(ct), nil
}

// Decrypt 解密 Encrypt 产生的密文。
func Decrypt(enc, secret string) (string, error) {
	data, err := hex.DecodeString(enc)
	if err != nil {
		return "", err
	}
	k := key(secret)
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := data[:ns], data[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
