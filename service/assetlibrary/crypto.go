package assetlibrary

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const channelCredentialVersion = "aes-gcm-v1"

type channelCredential struct {
	Secret string `json:"secret"`
}

func encryptChannelCredential(secret string) (string, error) {
	key, err := channelCredentialEncryptionKey()
	if err != nil {
		return "", err
	}
	plaintext, err := common.Marshal(channelCredential{Secret: secret})
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, plaintext, []byte("new-api/asset-channel-credential/v1"))
	return base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func decryptChannelCredential(payload string) (string, error) {
	key, err := channelCredentialEncryptionKey()
	if err != nil {
		return "", err
	}
	encoded, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decode asset channel credential: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(encoded) < aead.NonceSize() {
		return "", errors.New("asset channel credential payload is truncated")
	}
	plaintext, err := aead.Open(nil, encoded[:aead.NonceSize()], encoded[aead.NonceSize():], []byte("new-api/asset-channel-credential/v1"))
	if err != nil {
		return "", errors.New("asset channel credential cannot be decrypted")
	}
	var secret channelCredential
	if err = common.Unmarshal(plaintext, &secret); err != nil {
		return "", fmt.Errorf("decode asset channel credential data: %w", err)
	}
	return secret.Secret, nil
}

func channelCredentialEncryptionKey() ([]byte, error) {
	masterKey := strings.TrimSpace(os.Getenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY"))
	if masterKey == "" {
		masterKey = strings.TrimSpace(os.Getenv("CRYPTO_SECRET"))
	}
	if masterKey == "" {
		masterKey = strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	}
	if masterKey == "" {
		return nil, errors.New("set STORAGE_CREDENTIAL_ENCRYPTION_KEY, CRYPTO_SECRET, or SESSION_SECRET before saving asset channel credentials")
	}
	key := sha256.Sum256([]byte("new-api/asset-channel-credential/v1:" + masterKey))
	return key[:], nil
}

func credentialHint(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}
