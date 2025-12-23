package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestEncryptDecryptAES tests the basic AES encryption and decryption functions.
func TestEncryptDecryptAES(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32-byte key
	plaintext := []byte("this is a secret message")

	ciphertext, err := encryptAES(key, plaintext)
	if err != nil {
		t.Fatalf("encryptAES failed: %v", err)
	}

	decryptedText, err := decryptAES(key, ciphertext)
	if err != nil {
		t.Fatalf("decryptAES failed: %v", err)
	}

	if !bytes.Equal(plaintext, decryptedText) {
		t.Errorf("decrypted text does not match original plaintext. got %q, want %q", decryptedText, plaintext)
	}
}

// TestDeriveKey tests the key derivation function.
func TestDeriveKey(t *testing.T) {
	password := []byte("supersecretpassword")
	iterations := 100 // Use fewer iterations for testing speed
	keyLen := 32

	key1, salt1 := deriveKey(password, iterations, keyLen)
	key2, salt2 := deriveKey(password, iterations, keyLen)

	if len(key1) != keyLen {
		t.Errorf("derived key1 has wrong length: got %d, want %d", len(key1), keyLen)
	}
	if len(salt1) != 16 {
		t.Errorf("salt1 has wrong length: got %d, want %d", len(salt1), 16)
	}

	// Check that different salts produce different keys
	if bytes.Equal(key1, key2) {
		t.Errorf("derived keys should be different with different salts")
	}
	if bytes.Equal(salt1, salt2) {
		t.Errorf("salts should be different")
	}

	// Check that the same salt produces the same key (using pbkdf2 directly for test)
	// Note: deriveKey generates a random salt each time, so we can't directly test
	// determinism without exposing the salt generation or using pbkdf2 directly.
	// This part is implicitly tested by TestEncryptDecryptConfigFile.
}

// TestEncryptDecryptConfigFile tests encrypting and decrypting a config file.
func TestEncryptDecryptConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")
	encryptedPath := configPath + ".enc"
	password := []byte("testpassword")
	originalContent := []byte("[section]\nkey=value\n")

	// Create a dummy config file
	err := os.WriteFile(configPath, originalContent, 0600)
	if err != nil {
		t.Fatalf("Failed to write dummy config file: %v", err)
	}

	// Encrypt the file
	err = encryptConfigFile(configPath, password)
	if err != nil {
		t.Fatalf("encryptConfigFile failed: %v", err)
	}

	// Check if original file exists (it shouldn't if encryption succeeded without error)
	// Note: The original encryptConfigFile doesn't remove the source file, the command does.
	// _, err = os.Stat(configPath)
	// if err == nil {
	// 	t.Errorf("Original config file %q should not exist after encryption (based on command logic)", configPath)
	// }

	// Check if encrypted file exists
	if _, err := os.Stat(encryptedPath); os.IsNotExist(err) {
		t.Fatalf("Encrypted config file %q was not created", encryptedPath)
	}

	// Decrypt the file
	plaintext, err := decryptConfigFile(encryptedPath, password)
	if err != nil {
		t.Fatalf("decryptConfigFile failed: %v", err)
	}

	// Compare decrypted content with original
	if !bytes.Equal(plaintext, originalContent) { // Use bytes.Equal for []byte comparison
		t.Fatalf("decrypted content does not match original. Got %q, want %q", string(plaintext), string(originalContent))
	}

	// Check if encrypted file exists (it shouldn't if decryption succeeded without error)
	// Note: The original decryptConfigFile doesn't remove the source file, the command does.
	// _, err = os.Stat(encryptedPath)
	// if err == nil {
	// 	t.Errorf("Encrypted config file %q should not exist after decryption (based on command logic)", encryptedPath)
	// }

	// Check if decrypted file exists and content matches
	decryptedContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read decrypted config file: %v", err)
	}

	if !bytes.Equal(originalContent, decryptedContent) {
		t.Errorf("Decrypted content does not match original content. got %q, want %q", decryptedContent, originalContent)
	}
}

// TestConfigInitCmd tests the config init command.
func TestConfigInitCmd(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	err := os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd) // Change back at the end

	configPath := "config.json"

	// Ensure config doesn't exist initially
	os.Remove(configPath) // Ignore error if it doesn't exist

	cmd := configInitCmd()
	err = cmd.Exec(context.Background(), []string{})
	if err != nil {
		t.Fatalf("config init command failed: %v", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file %q was not created", configPath)
	}

	// Check content (basic check for template start)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read created config file: %v", err)
	}
	if !bytes.HasPrefix(content, []byte("{\n    \"targets\": {")) {
		t.Errorf("Config file content does not match expected template start. got %q", content)
	}

	// Test that it fails if the file already exists
	err = cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Errorf("config init command should have failed when config.json already exists")
	}
}

// TestLoadConfig tests loading an unencrypted config file.
func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	err := os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalWd)

	validConfig := []byte(`{
		"targets": {
			"bitbucket-cloud": {
				"my-bitbucket": {
					"username": "testuser",
					"apiToken": "secret123",
					"baseURL": "https://api.bitbucket.org"
				}
			}
		}
	}`)

	// Create unencrypted config file
	err = os.WriteFile(configFilename, validConfig, 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("loadConfig() failed: %v", err)
	}

	// Verify config was loaded correctly
	if config == nil {
		t.Fatal("loadConfig() returned nil config")
	}

	if len(config.Targets.BitbucketCloud) != 1 {
		t.Errorf("Expected 1 Bitbucket Cloud target, got %d", len(config.Targets.BitbucketCloud))
	}

	target, exists := config.Targets.BitbucketCloud["my-bitbucket"]
	if !exists {
		t.Fatal("Expected 'my-bitbucket' target to exist")
	}

	if target.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %q", target.Username)
	}

	if target.ApiToken != "secret123" {
		t.Errorf("Expected apiToken 'secret123', got %q", target.ApiToken)
	}

	if target.BaseURL != "https://api.bitbucket.org" {
		t.Errorf("Expected baseURL 'https://api.bitbucket.org', got %q", target.BaseURL)
	}
}
