package main

import (
	"fmt"
	"github.com/swayamchavan/algorand-fabric/engine/ba"
)

func main() {
	fmt.Printf("blockA: %x\n", ba.HashBlock([]byte("blockA")))
	fmt.Printf("blockB: %x\n", ba.HashBlock([]byte("blockB")))
	fmt.Printf("malicious-A: %x\n", ba.HashBlock([]byte("malicious-A")))
	fmt.Printf("malicious-B: %x\n", ba.HashBlock([]byte("malicious-B")))
}
