package chain

import (
	"fmt"
	"testing"
	"time"
)

func TestMerkleProofs(t *testing.T) {
	for n := 1; n <= 17; n++ {
		var hs []string
		for i := 0; i < n; i++ {
			hs = append(hs, Tx{Kind: "T", Nonce: fmt.Sprint(i)}.Hash())
		}
		root := MerkleRoot(hs)
		for i := range hs {
			if !VerifyProof(hs[i], Proof(hs, i), root) {
				t.Fatalf("n=%d i=%d proof failed", n, i)
			}
		}
		if n > 1 && VerifyProof(hs[0], Proof(hs, 1), root) {
			t.Fatal("wrong proof must fail")
		}
	}
}

func TestSignatures(t *testing.T) {
	s := NewSigner("dev")
	h := Header{Height: 1, PrevHash: GenesisPrev, MerkleRoot: MerkleRoot(nil), Time: time.Unix(1, 0)}.Hash()
	sig := s.Sign(h)
	if !VerifyBlock(s.PublicKey(), h, sig) {
		t.Fatal("valid signature rejected")
	}
	if VerifyBlock(s.PublicKey(), h[:len(h)-1]+"0", sig) {
		t.Fatal("tampered hash accepted")
	}
	if VerifyBlock(NewSigner("other").PublicKey(), h, sig) {
		t.Fatal("wrong key accepted")
	}
}
