package email

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
)

// Data represents the data needed for email encryption
type Data struct {
	EncryptedEmail string
	Challenge      string
	Difficulty     int
}

// GenerateData creates encrypted email data for the template
func GenerateData(email string) Data {
	challenge := generateChallenge()
	key := findProofOfWorkKey(challenge, 3)
	encryptedEmail := encryptEmail(email, key)

	return Data{
		EncryptedEmail: encryptedEmail,
		Challenge:      challenge,
		Difficulty:     3,
	}
}

// generateChallenge creates a random challenge string
func generateChallenge() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// findProofOfWorkKey finds a key that produces a hash with the required difficulty
func findProofOfWorkKey(challenge string, difficulty int) string {
	targetPrefix := ""
	for range difficulty {
		targetPrefix += "0"
	}

	for i := 0; ; i++ {
		key := strconv.Itoa(i)
		hash := generateHash(challenge + key)
		if hash[:difficulty] == targetPrefix {
			return key
		}
	}
}

// generateHash creates a SHA256 hash of the input string
func generateHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// encryptEmail encrypts an email using AES-GCM
func encryptEmail(email string, key string) string {
	keyHash := sha256.Sum256([]byte(key))

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		panic(err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(email), nil)
	return hex.EncodeToString(ciphertext)
}
