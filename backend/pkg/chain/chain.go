// Package chain implements the primitives of OMM Chain, the game's own
// asset ledger: canonical transaction hashing, Merkle trees with inclusion
// proofs, and ed25519-signed, hash-linked block headers. Anyone holding the
// public key can verify that a transaction is in a block and that blocks
// were produced by the game server and never rewritten.
package chain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Tx is the hashed content of a transaction.
type Tx struct {
	Kind     string          `json:"kind"`
	TokenID  int64           `json:"token_id,omitempty"`
	From     string          `json:"from,omitempty"`
	To       string          `json:"to,omitempty"`
	Amount   int64           `json:"amount,omitempty"`
	Currency string          `json:"currency,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Time     int64           `json:"time"` // unix nanoseconds
	Nonce    string          `json:"nonce"`
}

// Hash is sha256 over the canonical JSON (struct field order is fixed).
func (t Tx) Hash() string {
	b, _ := json.Marshal(t)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func node(l, r []byte) []byte {
	h := sha256.New()
	h.Write([]byte{1})
	h.Write(l)
	h.Write(r)
	return h.Sum(nil)
}

func leaf(txHash string) []byte {
	b, _ := hex.DecodeString(txHash)
	h := sha256.New()
	h.Write([]byte{0}) // domain separation: leaves can't be confused with nodes
	h.Write(b)
	return h.Sum(nil)
}

// MerkleRoot of tx hashes (hex). Odd levels duplicate their last node.
func MerkleRoot(txHashes []string) string {
	if len(txHashes) == 0 {
		z := sha256.Sum256(nil)
		return hex.EncodeToString(z[:])
	}
	level := make([][]byte, len(txHashes))
	for i, h := range txHashes {
		level[i] = leaf(h)
	}
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([][]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			next = append(next, node(level[i], level[i+1]))
		}
		level = next
	}
	return hex.EncodeToString(level[0])
}

// Step is one sibling on the path from a leaf to the root.
type Step struct {
	Hash string `json:"hash"`
	Left bool   `json:"left"`
}

// Proof returns the inclusion proof of txHashes[index].
func Proof(txHashes []string, index int) []Step {
	level := make([][]byte, len(txHashes))
	for i, h := range txHashes {
		level[i] = leaf(h)
	}
	var out []Step
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		sib := index ^ 1
		out = append(out, Step{Hash: hex.EncodeToString(level[sib]), Left: sib < index})
		next := make([][]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			next = append(next, node(level[i], level[i+1]))
		}
		level = next
		index /= 2
	}
	return out
}

// VerifyProof checks a tx hash against a Merkle root.
func VerifyProof(txHash string, proof []Step, root string) bool {
	cur := leaf(txHash)
	for _, s := range proof {
		sib, err := hex.DecodeString(s.Hash)
		if err != nil {
			return false
		}
		if s.Left {
			cur = node(sib, cur)
		} else {
			cur = node(cur, sib)
		}
	}
	return hex.EncodeToString(cur) == root
}

// Header is the signed part of a block.
type Header struct {
	Height     int64
	PrevHash   string
	MerkleRoot string
	TxCount    int
	Time       time.Time
}

func (h Header) canonical() []byte {
	return []byte(fmt.Sprintf("omm-chain|%d|%s|%s|%d|%s", h.Height, h.PrevHash, h.MerkleRoot, h.TxCount, strconv.FormatInt(h.Time.UnixNano(), 10)))
}

// Hash of the header.
func (h Header) Hash() string {
	s := sha256.Sum256(h.canonical())
	return hex.EncodeToString(s[:])
}

// Signer holds the chain's ed25519 key.
type Signer struct {
	priv ed25519.PrivateKey
}

// NewSigner derives the key from a 32-byte seed (hex or any string, hashed).
func NewSigner(seedText string) *Signer {
	var s [32]byte
	if b, err := hex.DecodeString(seedText); err == nil && len(b) == 32 {
		copy(s[:], b)
	} else {
		s = sha256.Sum256([]byte("omm-chain-key|" + seedText))
	}
	return &Signer{priv: ed25519.NewKeyFromSeed(s[:])}
}

func (s *Signer) PublicKey() string { return hex.EncodeToString(s.priv.Public().(ed25519.PublicKey)) }

// Sign signs a block hash.
func (s *Signer) Sign(blockHash string) string {
	return hex.EncodeToString(ed25519.Sign(s.priv, []byte(blockHash)))
}

// VerifyBlock checks the signature of a block hash with a public key.
func VerifyBlock(publicKeyHex, blockHash, sigHex string) bool {
	pk, err1 := hex.DecodeString(publicKeyHex)
	sig, err2 := hex.DecodeString(sigHex)
	if err1 != nil || err2 != nil || len(pk) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(pk, []byte(blockHash), sig)
}

// GenesisPrev is the previous hash of block 0.
const GenesisPrev = "0000000000000000000000000000000000000000000000000000000000000000"
