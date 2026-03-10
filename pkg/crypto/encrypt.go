package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// EncryptedData is the JSON structure stored in the database for encrypted values.
// It is compatible with the existing format used by the key management provider.
type EncryptedData struct {
	IV      string `json:"iv"`
	Data    string `json:"data"`
	AuthTag string `json:"authTag"`
}

// Encryptor provides AES-256-GCM encryption and decryption using a shared key.
type Encryptor struct {
	key []byte
}

// NewEncryptor creates an Encryptor from a hex-encoded key string.
func NewEncryptor(hexKey string) (*Encryptor, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (256 bits), got %d", len(key))
	}
	return &Encryptor{key: key}, nil
}

// NewEncryptorFromBytes creates an Encryptor from raw key bytes.
func NewEncryptorFromBytes(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (256 bits), got %d", len(key))
	}
	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a JSON string.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)

	data := EncryptedData{
		IV:      base64.StdEncoding.EncodeToString(nonce),
		Data:    base64.StdEncoding.EncodeToString(ciphertext[:len(ciphertext)-16]),
		AuthTag: base64.StdEncoding.EncodeToString(ciphertext[len(ciphertext)-16:]),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	return string(jsonData), nil
}

// Decrypt decrypts a JSON-encoded encrypted string produced by Encrypt.
func (e *Encryptor) Decrypt(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	var data EncryptedData
	if err := json.Unmarshal([]byte(encrypted), &data); err != nil {
		return "", fmt.Errorf("failed to unmarshal encrypted data: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(data.IV)
	if err != nil {
		return "", fmt.Errorf("failed to decode IV: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(data.Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	authTag, err := base64.StdEncoding.DecodeString(data.AuthTag)
	if err != nil {
		return "", fmt.Errorf("failed to decode auth tag: %w", err)
	}

	fullCiphertext := append(ciphertext, authTag...)

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := aesGCM.Open(nil, nonce, fullCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a string looks like it was encrypted by this package
// (i.e., is valid JSON with the expected fields). This enables backwards-compatible
// migration from plaintext to encrypted values.
func IsEncrypted(s string) bool {
	if s == "" {
		return false
	}
	var data EncryptedData
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return false
	}
	return data.IV != "" && data.Data != "" && data.AuthTag != ""
}
