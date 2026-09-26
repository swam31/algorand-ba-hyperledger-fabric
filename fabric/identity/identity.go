package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
)

// LoadOrGenerateKey reads an ed25519 seed from keyFile.
// If the file does not exist, it generates a new keypair, saves the seed to keyFile
// with 0600 permissions, and logs the creation.
func LoadOrGenerateKey(keyFile string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if keyFile == "" {
		return nil, nil, fmt.Errorf("keyFile path is empty")
	}

	data, err := os.ReadFile(keyFile)
	if err == nil {
		// File exists, decode hex seed
		seed, err := hex.DecodeString(string(data))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode hex seed from %s: %w", keyFile, err)
		}
		if len(seed) != ed25519.SeedSize {
			return nil, nil, fmt.Errorf("invalid seed length in %s: expected %d, got %d", keyFile, ed25519.SeedSize, len(seed))
		}
		
		priv := ed25519.NewKeyFromSeed(seed)
		pub := priv.Public().(ed25519.PublicKey)
		return pub, priv, nil
	}

	if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to read key file %s: %w", keyFile, err)
	}

	// File does not exist, generate a new key
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	// Extract the 32-byte seed from the private key
	seed := priv.Seed()
	hexSeed := hex.EncodeToString(seed)

	if err := os.WriteFile(keyFile, []byte(hexSeed), 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write key file %s: %w", keyFile, err)
	}

	fmt.Printf(">>> [ALGORAND BA*] Created new identity. Saved seed to %s\n", keyFile)
	return pub, priv, nil
}
