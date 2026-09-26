package identity

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerateKey_EmptyPath(t *testing.T) {
	_, _, err := LoadOrGenerateKey("")
	if err == nil {
		t.Fatal("Expected error for empty path")
	}
}

func TestLoadOrGenerateKey_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.hex")

	// 1. Generate
	pub1, priv1, err := LoadOrGenerateKey(keyFile)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	if len(pub1) != ed25519.PublicKeySize || len(priv1) != ed25519.PrivateKeySize {
		t.Fatal("Invalid key sizes generated")
	}

	// Verify permissions
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("Expected permissions 0600, got %v", info.Mode().Perm())
	}

	// 2. Load
	pub2, priv2, err := LoadOrGenerateKey(keyFile)
	if err != nil {
		t.Fatalf("Failed to load key: %v", err)
	}

	if !bytes.Equal(pub1, pub2) || !bytes.Equal(priv1, priv2) {
		t.Fatal("Loaded key does not match generated key")
	}
}

func TestLoadOrGenerateKey_MalformedHex(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.hex")

	err := os.WriteFile(keyFile, []byte("not-hex-data!"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = LoadOrGenerateKey(keyFile)
	if err == nil {
		t.Fatal("Expected error for malformed hex")
	}
}

func TestLoadOrGenerateKey_WrongLength(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.hex")

	err := os.WriteFile(keyFile, []byte("aabbccdd"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = LoadOrGenerateKey(keyFile)
	if err == nil {
		t.Fatal("Expected error for wrong length seed")
	}
}
