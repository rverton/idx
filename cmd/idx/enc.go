package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

func encryptConfigFile(path string, pw []byte) error {
	derivedKey, salt := deriveKey(pw, 10000, 32)

	// read config file raw
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	// encrypt config file
	encryptedContent, err := encryptAES(derivedKey, content)
	if err != nil {
		return fmt.Errorf("encrypt config file: %w", err)
	}

	// write encrypted config file
	newPath := path + ".enc"
	encFile, err := os.Create(newPath)
	if err != nil {
		return fmt.Errorf("create encrypted config file: %w", err)
	}
	defer encFile.Close()

	// write salt to the first 16 bytes of the file
	if _, err := encFile.Write(salt); err != nil {
		return fmt.Errorf("write salt to encrypted config file: %w", err)
	}

	// write encrypted content to the file
	if _, err := encFile.Write(encryptedContent); err != nil {
		return fmt.Errorf("write encrypted config file: %w", err)
	}

	return nil
}

func decryptConfigFile(path string, key []byte) ([]byte, error) {
	// read encrypted config file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read encrypted config file: %w", err)
	}

	if len(content) < 16 {
		return nil, fmt.Errorf("encrypted config file is too short")
	}

	salt := content[:16]
	content = content[16:]

	derivedKey := pbkdf2.Key(key, salt, 10000, 32, sha256.New)

	plaintext, err := decryptAES(derivedKey, content)
	if err != nil {
		return nil, fmt.Errorf("decrypt config file: %w", err)
	}
	return plaintext, nil
}

// deriveKey derives a key from the password using PBKDF2 with SHA256.
func deriveKey(password []byte, iterations, keyLen int) ([]byte, []byte) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	return pbkdf2.Key(password, salt, iterations, keyLen, sha256.New), salt
}

func readPasswordSafe(confirm bool) ([]byte, error) {
	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	fmt.Println() // Add a newline after password input

	if confirm {
		fmt.Print("Confirm password: ")
		passwordConfirm, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("read confirmation password: %w", err)
		}
		fmt.Println() // Add a newline after confirmation password input

		if !bytes.Equal(password, passwordConfirm) {
			return nil, fmt.Errorf("passwords do not match")
		}
	}

	return password, nil
}

func encryptAES(key []byte, plaintext []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decryptAES(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
