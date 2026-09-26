// Package vrf provides the Verifiable Random Function used by Algorand's
// cryptographic sortition (paper section 5).
//
// A VRF gives you, from a secret key and an input:
//
//	hash  - looks uniformly random, but is UNIQUELY determined by (sk, input).
//	        The holder cannot choose, grind, or bias it.
//	proof - lets anyone holding the matching PUBLIC key verify that hash really
//	        is the correct output for that input, without learning sk.
//
// That combination is what lets a node privately discover it was selected for a
// committee and only later prove it to everyone else.
package vrf

import (
	"crypto/ed25519"
	"crypto/sha512"
)

// Size constants.
const (
	HashLen  = 64 // bytes of VRF output (sha512)
	HashBits = HashLen * 8
)

// VRF abstracts the primitive so the implementation can be swapped.
//
// The Algorand paper uses an elliptic-curve VRF over Curve25519 (Goldberg et
// al.). We ship Ed25519VRF below, which is adequate for a permissioned lab
// network; a standards-track ECVRF (draft-irtf-cfrg-vrf) can be dropped in here
// without touching any other file.
type VRF interface {
	// Prove returns the VRF hash and its proof for input m.
	Prove(sk ed25519.PrivateKey, m []byte) (hash []byte, proof []byte)

	// Verify checks that hash/proof are the genuine VRF output of pk on m.
	Verify(pk ed25519.PublicKey, m, hash, proof []byte) bool
}

// Ed25519VRF implements VRF using Ed25519's deterministic signature as the
// proof, and SHA-512 of that signature as the pseudorandom output.
//
// Why this works: RFC 8032 Ed25519 signing is fully deterministic. For a fixed
// (sk, m) there is exactly one signature an honest signer produces, so hashing
// it yields a value the signer cannot influence, and Verify is just signature
// verification plus a hash comparison.
//
// LIMITATION - state this in any writeup. This is not a cryptographically
// rigorous VRF. Ed25519 does not guarantee signature UNIQUENESS in the strict
// sense: a malicious key holder using non-canonical scalar encodings or
// small-order points can, in some verifier implementations, produce more than
// one signature that verifies. A real VRF proves uniqueness unconditionally,
// even for adversarially generated keys - which is exactly the property
// Algorand's security argument leans on. For a permissioned Fabric network
// where orderer keys are issued by a CA under an MSP, the gap is acceptable.
// For anything adversarial, swap in a true ECVRF.
type Ed25519VRF struct{}

func (Ed25519VRF) Prove(sk ed25519.PrivateKey, m []byte) (hash, proof []byte) {
	proof = ed25519.Sign(sk, m)
	h := sha512.Sum512(proof)
	return h[:], proof
}

func (Ed25519VRF) Verify(pk ed25519.PublicKey, m, hash, proof []byte) bool {
	if len(proof) != ed25519.SignatureSize || len(hash) != HashLen {
		return false
	}
	if !ed25519.Verify(pk, m, proof) {
		return false
	}
	h := sha512.Sum512(proof)
	// constant-time not required: both values are public
	for i := range h {
		if h[i] != hash[i] {
			return false
		}
	}
	return true
}

// Default is the VRF used throughout the engine.
var Default VRF = Ed25519VRF{}
