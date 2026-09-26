package main

import (
	"fmt"
	"github.com/swayamchavan/algorand-fabric/engine/ba"
	"crypto/ed25519"
)

func main() {
	pub, priv, _ := ed25519.GenerateKey(nil)
	v := &ba.Vote{
		Voter: pub,
		Round: 1,
		Step: 2,
		Value: []byte("test"),
	}
	v.Sign(priv)
	
	msg := v.Encode()
	v2, err := ba.DecodeVote(msg)
	if err != nil {
		fmt.Println("Decode error:", err)
		return
	}
	fmt.Println("Verify:", v2.VerifySignature())
}
