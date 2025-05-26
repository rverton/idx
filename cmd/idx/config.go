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
	"idx/internal/targets/bitbucket" // Added Bitbucket import
	"idx/internal/targets/gitlab"    // Added GitLab import
	"idx/internal/targets/smb"
	"io"
	"log"
	"os"
	"syscall"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"github.com/peterbourgon/ff/v4"
	"golang.org/x/term"
)

var configFile = "config.json"
var configFileEnc = "config.json.enc"

type Config struct {
	Targets struct {
		Smb       map[string]TargetSMBConfig       `json:"smb"`
		Bitbucket map[string]TargetBitbucketConfig `json:"bitbucket"`
		Gitlab    map[string]TargetGitlabConfig    `json:"gitlab"`
	} `json:"targets"`
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

func (t TargetSMBConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetSMBConfig
	return json.Marshal(&struct {
		*Alias
		Password string `json:"password"`
	}{
		Alias:    (*Alias)(&t),
		Password: "REDACTED",
	})
}

// TargetBitbucketConfig defines the configuration for a Bitbucket target.
type TargetBitbucketConfig struct {
	Username    string `json:"username"`
	AppPassword string `json:"appPassword"`
	BaseURL     string `json:"baseURL,omitempty"` // Optional: Defaults to Bitbucket Cloud API
}

func (t TargetBitbucketConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetBitbucketConfig
	return json.Marshal(&struct {
		*Alias
		AppPassword string `json:"appPassword"`
	}{
		Alias:       (*Alias)(&t),
		AppPassword: "REDACTED",
	})
}

// TargetGitlabConfig defines the configuration for a GitLab target.
type TargetGitlabConfig struct {
	AccessToken string `json:"accessToken"`
	BaseURL     string `json:"baseURL,omitempty"` // Optional: Defaults to https://gitlab.com/api/v4
}

func (t TargetGitlabConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetGitlabConfig
	return json.Marshal(&struct {
		*Alias
		AccessToken string `json:"accessToken"`
	}{
		Alias:       (*Alias)(&t),
		AccessToken: "REDACTED",
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
			if _, err := os.Stat(configFile); err != nil {
				return fmt.Errorf("config file not found: %w", err)
			}

			content, err := os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("failed to read config file: %w", err)
			}

			config, err := parseConfig(content)
			if err != nil {
				return fmt.Errorf("failed to parse config file: %w", err)
			}

			if *useJSON {
				targets := config.Targets
				jsonOutput, err := json.MarshalIndent(targets, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal config to JSON: %w", err)
				}
				fmt.Println(string(jsonOutput))
				return nil
			} else {
				fmt.Println("Targets:")
				for name := range config.Targets.Smb {
					fmt.Printf("- SMB Target: %s\n", name)
				}
				for name := range config.Targets.Bitbucket {
					fmt.Printf("- Bitbucket Target: %s\n", name)
				}
				for name := range config.Targets.Gitlab {
					fmt.Printf("- GitLab Target: %s\n", name)
				}
			}

			return nil
		},
	}
}

func configVerifyCmd() *ff.Command {
	flags := ff.NewFlagSet("verify").SetParent(rootFlags)
	// targetFlag := flags.String('t', "target", "", "Specify a target to verify (optional)")
	return &ff.Command{
		Name:      "verify",
		Usage:     "idx config verify [target]",
		ShortHelp: "Verifies the config file structure and tests connections",
		Flags:     flags,
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

			result := verifyTargets(ctx, config)
			if *useJSON {
				jsonOutput, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal verification results to JSON: %w", err)
				}
				fmt.Println(string(jsonOutput))
			} else {
				for _, res := range result {
					if res.Success {
						fmt.Printf("%s (%s) verified successfully in %s\n", res.Name, res.Type, res.Duration)
					} else {
						fmt.Printf("%s (%s) verification failed: %v (Duration: %s)\n", res.Name, res.Type, res.Error, res.Duration)
					}
				}
			}

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

func verifyTargets(ctx context.Context, config *Config) []verificationResult {
	var results []verificationResult
	for name, target := range config.Targets.Smb {
		startTime := time.Now()

		conn, err := smb.Connect(target.Host, target.Port, target.Username, target.Password)
		if err != nil {
			results = append(results,
				verificationResult{
					Name:     name,
					Type:     "smb",
					Success:  false,
					Error:    err,
					Duration: time.Since(startTime),
				},
			)
			continue
		}

		if conn.IsAuthenticated() {
			results = append(results, verificationResult{
				Name:    name,
				Type:    "smb",
				Success: true,
			})
		} else {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "smb",
				Success:  false,
				Error:    fmt.Errorf("failed to authenticate to smb target"),
				Duration: time.Since(startTime),
			})
		}
		conn.Close()
	}

	for name, target := range config.Targets.Bitbucket {
		client, err := bitbucket.NewAPIClient(target.BaseURL, target.Username, target.AppPassword)
		if err != nil {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "bitbucket",
				Success:  false,
				Error:    err,
				Duration: time.Since(time.Now()),
			})
			continue
		}

		if err := client.VerifyConnection(ctx); err != nil {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "bitbucket",
				Success:  false,
				Error:    err,
				Duration: time.Since(time.Now()),
			})
		} else {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "bitbucket",
				Success:  true,
				Duration: time.Since(time.Now()),
			})
		}
	}

	for name, target := range config.Targets.Gitlab {
		client, err := gitlab.NewAPIClient(target.BaseURL, target.AccessToken)
		if err != nil {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "gitlab",
				Success:  false,
				Error:    err,
				Duration: time.Since(time.Now()),
			})
			continue
		}
		if err := client.VerifyConnection(ctx); err != nil {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "gitlab",
				Success:  false,
				Error:    err,
				Duration: time.Since(time.Now()),
			})
		} else {
			results = append(results, verificationResult{
				Name:     name,
				Type:     "gitlab",
				Success:  true,
				Duration: time.Since(time.Now()),
			})
		}
	}

	return results
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
