package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"syscall"

	"golang.org/x/crypto/pbkdf2"

	"github.com/peterbourgon/ff/v4"
	"golang.org/x/term"
)

const tplConfig = `; [example-target-smb]
; type = "smb"
; username = "user"
; password = "password"
; host = "localhost"
; port = 445
; server = "127.0.0.1/32"
; shares = "*"
; exclude-shares = "share1,share2"
; exlucde-extensions = "exe,bin"
`

func configCmd() *ff.Command {
	return &ff.Command{
		Name:      "config",
		Usage:     "idx config <subcommand>",
		ShortHelp: "repeatedly print the first argument to stdout",
		Subcommands: []*ff.Command{
			configInitCmd(),
			configEncryptCmd(),
			configDecryptCmd(),
		},
	}
}

func configInitCmd() *ff.Command {
	return &ff.Command{
		Name:      "init",
		Usage:     "idx config init",
		ShortHelp: "creates a new (unencrypted) config file",
		Exec: func(ctx context.Context, args []string) error {
			if _, err := os.Stat("config.ini"); err == nil {
				return fmt.Errorf("config.ini already exists")
			}

			initFile, err := os.Create("config.ini")
			if err != nil {
				return fmt.Errorf("failed to create config.ini: %w", err)
			}
			defer initFile.Close()

			_, err = initFile.WriteString(tplConfig)
			if err != nil {
				return fmt.Errorf("failed to write to config.ini: %w", err)
			}

			return nil
		},
	}
}

func configEncryptCmd() *ff.Command {
	encFlags := ff.NewFlagSet("encrypt")
	return &ff.Command{
		Name:      "encrypt",
		Usage:     "idx config encrypt",
		ShortHelp: "encrypts the config file and removes the unencrypted one",
		Flags:     encFlags,
		Exec: func(ctx context.Context, args []string) error {
			pw, err := readPasswordSafe()
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := encryptConfigFile("config.ini", pw); err != nil {
				return fmt.Errorf("failed to encrypt config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove("config.ini"); err != nil {
				return fmt.Errorf("failed to remove unencrypted config file: %w", err)
			}

			return nil
		},
	}
}

func configDecryptCmd() *ff.Command {
	decFlags := ff.NewFlagSet("decrypt")
	return &ff.Command{
		Name:      "decrypt",
		Usage:     "idx config decrypt",
		ShortHelp: "decrypts the config file and removes the encrypted one",
		Flags:     decFlags,
		Exec: func(ctx context.Context, args []string) error {
			pw, err := readPasswordSafe()
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := decryptConfigFile("config.ini.enc", pw); err != nil {
				return fmt.Errorf("failed to decrypt config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove("config.ini.enc"); err != nil {
				return fmt.Errorf("failed to remove unencrypted config file: %w", err)
			}

			return nil
		},
	}
}

func encryptConfigFile(path string, pw []byte) error {
	// 32bytes, 10000 iterations
	derivedKey, salt := deriveKey(pw, 10000, 32)

	// read config file raw
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// encrypt config file
	encryptedContent, err := encryptAES(derivedKey, content)
	if err != nil {
		return fmt.Errorf("failed to encrypt config file: %w", err)
	}

	// write encrypted config file
	newPath := path + ".enc"
	encFile, err := os.Create(newPath)
	if err != nil {
		return fmt.Errorf("failed to create encrypted config file: %w", err)
	}
	defer encFile.Close()

	// write salt to the first 16 bytes of the file
	if _, err := encFile.Write(salt); err != nil {
		return fmt.Errorf("failed to write salt to encrypted config file: %w", err)
	}

	// write encrypted content to the file
	if _, err := encFile.Write(encryptedContent); err != nil {
		return fmt.Errorf("failed to write encrypted config file: %w", err)
	}

	return nil
}

func decryptConfigFile(path string, key []byte) error {
	// read encrypted config file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read encrypted config file: %w", err)
	}

	if len(content) < 16 {
		return fmt.Errorf("encrypted config file is too short")
	}

	salt := content[:16]
	content = content[16:]

	derivedKey := pbkdf2.Key(key, salt, 10000, 32, sha256.New)

	plaintext, err := decryptAES(derivedKey, content)
	if err != nil {
		return fmt.Errorf("failed to decrypt config file: %w", err)
	}

	// write decrypted config file
	newPath := "config.ini"
	decFile, err := os.Create(newPath)
	if err != nil {
		return fmt.Errorf("failed to create decrypted config file: %w", err)
	}

	defer decFile.Close()
	if _, err := decFile.Write(plaintext); err != nil {
		return fmt.Errorf("failed to write decrypted config file: %w", err)
	}

	return nil
}

// deriveKey derives a key from the password using PBKDF2 with SHA256.
func deriveKey(password []byte, iterations, keyLen int) ([]byte, []byte) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	return pbkdf2.Key(password, salt, iterations, keyLen, sha256.New), salt
}

func readPasswordSafe() ([]byte, error) {
	fmt.Print("Enter password to encrypt config file: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return []byte{}, err
	}
	fmt.Println()

	return password, nil
}

func encryptAES(key []byte, plaintext []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decryptAES(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
