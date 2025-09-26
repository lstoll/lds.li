package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
)

func main() {
	difficulty := flag.Int("difficulty", 3, "Number of leading zeros required")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <email>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	email := flag.Arg(0)

	challenge := generateChallenge()
	key := findProofOfWorkKey(challenge, *difficulty)
	encryptedEmail := encryptEmail(email, key)

	fmt.Printf("Found key: %s\n", key)
	fmt.Printf("Challenge: %s\n", challenge)
	fmt.Printf("Hash: %s\n", generateHash(challenge+key))
	fmt.Printf("\nEncrypted email: %s\n", encryptedEmail)

	fmt.Printf("\nFor JavaScript:\n")
	fmt.Printf("const encryptedEmail = '%s';\n", encryptedEmail)
	fmt.Printf("const challenge = '%s';\n", challenge)
	fmt.Printf("const difficulty = %d;\n", *difficulty)
}

func generateChallenge() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

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

func generateHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

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
