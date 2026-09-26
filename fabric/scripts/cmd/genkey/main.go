package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/swayamchavan/algorand-fabric/fabric/identity"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <key-file>", os.Args[0])
	}
	keyFile := os.Args[1]

	pub, _, err := identity.LoadOrGenerateKey(keyFile)
	if err != nil {
		log.Fatalf("Failed to load or generate key: %v", err)
	}

	fmt.Print(hex.EncodeToString(pub))
}
