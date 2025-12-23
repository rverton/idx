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
	"log/slog"
	"os"
	"syscall"
	"time"

	bitbucketcloud "idx/targets/bitbucket-cloud"
	bitbucketdc "idx/targets/bitbucket-dc"

	"github.com/peterbourgon/ff/v4"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

const (
	configFilename    = "config.json"
	configFilenameEnc = "config.json.enc"
)

type Config struct {
	Targets struct {
		BitbucketCloud map[string]TargetBitbucketConfig `json:"bitbucket-cloud"`
		BitbucketDC    map[string]TargetBitbucketConfig `json:"bitbucket-dc"`
	} `json:"targets"`
}

// TargetBitbucketConfig defines the configuration for a Bitbucket target.
type TargetBitbucketConfig struct {
	Username string `json:"username"`
	ApiToken string `json:"apiToken"`
	BaseURL  string `json:"baseURL"` // unused for Bitbucket Cloud

	Workspaces []string `json:"workspaces"`
}

// MarshalJSON customizes the JSON marshaling to redact the ApiToken field.
func (t TargetBitbucketConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetBitbucketConfig
	return json.Marshal(&struct {
		*Alias
		ApiToken string `json:"apiToken"`
	}{
		Alias:    (*Alias)(&t),
		ApiToken: "REDACTED",
	})
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
			configListTargetsCmd(),
		},
	}
}

func configListTargetsCmd() *ff.Command {
	flags := ff.NewFlagSet("targets-list").SetParent(rootFlags)
	return &ff.Command{
		Name:      "targets-list",
		Usage:     "idx config targets-list",
		ShortHelp: "Lists all targets in the config file",
		Flags:     flags,
		Exec: func(ctx context.Context, args []string) error {
			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println("Targets:")
			for name := range config.Targets.BitbucketCloud {
				fmt.Printf("- Bitbucket Cloud: %s\n", name)
			}
			for name := range config.Targets.BitbucketDC {
				fmt.Printf("- Bitbucket Data Center: %s\n", name)
			}

			return nil
		},
	}
}

func configVerifyCmd() *ff.Command {
	flags := ff.NewFlagSet("verify").SetParent(rootFlags)
	return &ff.Command{
		Name:      "verify",
		Usage:     "idx config verify [target]",
		ShortHelp: "Verifies the config file structure and tests connections",
		Flags:     flags,
		Exec: func(ctx context.Context, args []string) error {
			config, err := LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			verifyTargets(ctx, config)

			return nil
		},
	}
}

type verificationResult struct {
	Name     string
	Type     string
	Success  bool
	Error    error
	Duration time.Duration
}

func verifyTargets(ctx context.Context, config *Config) {
	// Verify Bitbucket Cloud targets
	for name, target := range config.Targets.BitbucketCloud {
		client, err := bitbucketcloud.NewAPIClient(target.Username, target.ApiToken)
		if err != nil {
			slog.Error("failed to create Bitbucket Cloud client", "target", name, "error", err)
			continue
		}

		if err := client.VerifyConnection(ctx); err != nil {
			slog.Error(
				"Bitbucket Cloud target verification failed",
				"target",
				name,
				"username",
				target.Username,
				"len(apiToken)",
				len(target.ApiToken),
				"error",
				err,
			)
		} else {
			slog.Info(
				"Bitbucket Cloud target verification succeeded",
				"target",
				name,
				"username",
				target.Username,
				"len(apiToken)",
				len(target.ApiToken),
			)
		}
	}

	// Verify Bitbucket Data Center targets
	for name, target := range config.Targets.BitbucketDC {
		client, err := bitbucketdc.NewAPIClient(target.BaseURL, target.Username, target.ApiToken)
		if err != nil {
			slog.Error("failed to create Bitbucket DC client", "target", name, "error", err)
			continue
		}

		if err := client.VerifyConnection(ctx); err != nil {
			slog.Error(
				"Bitbucket DC target verification failed",
				"target",
				name,
				"username",
				target.Username,
				"len(apiToken)",
				len(target.ApiToken),
				"error",
				err,
			)
		} else {
			slog.Info(
				"Bitbucket DC target verification succeeded",
				"target",
				name,
				"username",
				target.Username,
				"len(apiToken)",
				len(target.ApiToken),
			)
		}
	}
}

func configInitCmd() *ff.Command {
	return &ff.Command{
		Name:      "init",
		Usage:     "idx config init",
		ShortHelp: "creates a new (unencrypted) config file",
		Exec: func(ctx context.Context, args []string) error {
			if _, err := os.Stat(configFilename); err == nil {
				return fmt.Errorf("%v already exists", configFilename)
			}

			initFile, err := os.Create(configFilename)
			if err != nil {
				return fmt.Errorf("failed to create %v: %w", configFilename, err)
			}
			defer initFile.Close()

			_, err = initFile.WriteString(tplConfig)
			if err != nil {
				return fmt.Errorf("failed to write to %v: %w", configFilename, err)
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

			if err := encryptConfigFile(configFilename, pw); err != nil {
				return fmt.Errorf("failed to encrypt config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove(configFilename); err != nil {
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

			plaintext, err := decryptConfigFile(configFilenameEnc, pw)
			if err != nil {
				return fmt.Errorf("failed to decrypt config file: %w", err)
			}

			// write decrypted config file
			decFile, err := os.Create(configFilename)
			if err != nil {
				return fmt.Errorf("create decrypted config file: %w", err)
			}

			defer decFile.Close()
			if _, err := decFile.Write(plaintext); err != nil {
				return fmt.Errorf("write decrypted config file: %w", err)
			}

			// remove unencrypted config file
			if err := os.Remove(configFilenameEnc); err != nil {
				return fmt.Errorf("failed to remove unencrypted config file: %w", err)
			}

			return nil
		},
	}
}

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

func parseConfig(content []byte) (*Config, error) {
	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}

func LoadConfig() (*Config, error) {
	var plaintext []byte
	var err error

	// first check if there is an unencrypted config file
	// and use it if available
	if _, err = os.Stat(configFilename); err == nil {
		log.Printf("warning: using unencrypted %v", configFilename)
		plaintext, err = os.ReadFile(configFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// if not, try to read the encrypted config file
	} else {
		pw, err := readPasswordSafe(false)
		if err != nil {
			return nil, fmt.Errorf("failed to read password: %w", err)
		}

		if plaintext, err = decryptConfigFile(configFilenameEnc, pw); err != nil {
			return nil, fmt.Errorf("failed to decrypt config file: %w", err)
		}
	}

	return parseConfig(plaintext)
}
