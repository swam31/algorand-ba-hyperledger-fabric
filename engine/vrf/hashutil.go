package vrf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

// subUserHash derives a per-sub-user hash H(sorthash || index).
//
// Used in two places in the paper:
//   - section 6, to give each of a proposer's sub-users its own priority draw
//   - Algorithm 9 (CommonCoin), so a node holding several seats contributes
//     several independent hashes to the minimum
func subUserHash(sortHash []byte, index uint64) []byte {
	h := sha256.New()
	h.Write(sortHash)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], index)
	h.Write(b[:])
	return h.Sum(nil)
}

// SubUserHash is the exported form, needed by the CommonCoin implementation.
func SubUserHash(sortHash []byte, index uint64) []byte { return subUserHash(sortHash, index) }

// less reports whether a sorts before b as a big-endian unsigned integer.
func less(a, b []byte) bool { return bytes.Compare(a, b) < 0 }

// Less is the exported form.
func Less(a, b []byte) bool { return less(a, b) }
