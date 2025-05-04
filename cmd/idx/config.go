package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"golang.org/x/crypto/pbkdf2"

	"github.com/peterbourgon/ff/v4"
	"golang.org/x/term"
)

var configFile = "config.json"
var configFileEnc = "config.json.enc"

type Config struct {
	Targets struct {
		Smb map[string]TargetSMBConfig `json:"smb"`
	} `json:"targets"`

	Notifications map[string]NotificationConfig `json:"notifications"`
}

type TargetSMBConfig struct {
	Username          string   `json:"username"`
	Password          string   `json:"password"`
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	Shares            string   `json:"shares"`
	ExcludeShares     []string `json:"exclude-shares"`
	ExcludeExtensions []string `json:"exclude-extensions"`
}

type NotificationConfig struct {
	Log struct {
		Output string `json:"output"`
	} `json:"log"`
}

func configCmd() *ff.Command {
	return &ff.Command{
		Name:      "config",
		Usage:     "idx config <subcommand>",
		ShortHelp: "Manage configuration file", // Updated ShortHelp
		Subcommands: []*ff.Command{
			configInitCmd(),
			configEncryptCmd(),
			configDecryptCmd(),
			configVerifyCmd(),
		},
	}
}

func configVerifyCmd() *ff.Command {
	return &ff.Command{
		Name:      "verify",
		Usage:     "idx config verify",
		ShortHelp: "Verifies the config file structure and tests connections",
		Exec: func(ctx context.Context, args []string) error {
			var plaintext []byte
			var err error

			if _, err = os.Stat(configFile); err == nil {
				log.Printf("%v found, using this instead of %v", configFile, configFileEnc)
				plaintext, err = os.ReadFile(configFile)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
			} else {
				pw, err := readPasswordSafe(false)
				if err != nil {
					return fmt.Errorf("failed to read password: %w", err)
				}

				if plaintext, err = decryptConfigFile(configFileEnc, pw); err != nil {
					return fmt.Errorf("failed to decrypt config file: %w", err)
				}
			}

			config, err := parseConfig(plaintext)
			if err != nil {
				return fmt.Errorf("failed to parse config: %w", err)
			}

			for name, target := range config.Targets.Smb {
				log.Printf("verifying target %s, %v:%v", name, target.Host, target.Port)
				// TODO: implement actual verification logic
			}

			return nil
		},
	}
}

func configInitCmd() *ff.Command {
	return &ff.Command{
		Name:      "init",
		Usage:     "idx config init",
		ShortHelp: "creates a new (unencrypted) config file",
		Exec: func(ctx context.Context, args []string) error {
			if _, err := os.Stat(configFile); err == nil {
				return fmt.Errorf("%v already exists", configFile)
			}

			initFile, err := os.Create(configFile)
			if err != nil {
				return fmt.Errorf("failed to create %v: %w", configFile, err)
			}
			defer initFile.Close()

			_, err = initFile.WriteString(tplConfig)
			if err != nil {
				return fmt.Errorf("failed to write to %v: %w", configFile, err)
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
			pw, err := readPasswordSafe(true)
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := encryptConfigFile(configFile, pw); err != nil {
				return fmt.Errorf("failed to encrypt config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove(configFile); err != nil {
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
			pw, err := readPasswordSafe(false)
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			plaintext, err := decryptConfigFile(configFileEnc, pw)
			if err != nil {
				return fmt.Errorf("failed to decrypt config file: %w", err)
			}

			// write decrypted config file
			decFile, err := os.Create(configFile)
			if err != nil {
				return fmt.Errorf("create decrypted config file: %w", err)
			}

			defer decFile.Close()
			if _, err := decFile.Write(plaintext); err != nil {
				return fmt.Errorf("write decrypted config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove(configFileEnc); err != nil {
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

func parseConfig(content []byte) (*Config, error) {
	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}
