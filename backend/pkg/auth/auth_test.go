package auth

import (
	"testing"
	"time"
)

func TestInitDataRoundTrip(t *testing.T) {
	now := time.Now()
	raw := SignInitData("123:abc", TelegramUser{ID: 42, FirstName: "Ali"}, now)
	d, err := ValidateInitData(raw, "123:abc", time.Hour, now)
	if err != nil || d.User.ID != 42 {
		t.Fatalf("valid data rejected: %v", err)
	}
	if _, err := ValidateInitData(raw, "other", time.Hour, now); err != ErrInitDataInvalid {
		t.Fatal("wrong token must fail")
	}
	if _, err := ValidateInitData(raw+"x", "123:abc", time.Hour, now); err == nil {
		t.Fatal("tampered data must fail")
	}
	if _, err := ValidateInitData(raw, "123:abc", time.Minute, now.Add(2*time.Minute)); err != ErrInitDataExpired {
		t.Fatal("expired must fail")
	}
}

// Known vector computed independently with Python's hmac module.
func TestInitDataKnownVector(t *testing.T) {
	raw := "auth_date=1700000000&query_id=Q1&user=%7B%22id%22%3A7%2C%22first_name%22%3A%22T%22%7D&hash=" + knownHash
	if _, err := ValidateInitData(raw, "7777:TEST", 0, time.Now()); err != nil {
		t.Fatalf("known vector failed: %v", err)
	}
}

func TestSessionTokens(t *testing.T) {
	iss := NewIssuer("s3cret", time.Hour)
	tok, _, err := iss.Issue(Session{AccountID: "a1", CharacterID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := iss.Verify(tok)
	if err != nil || s.AccountID != "a1" || s.CharacterID != "c1" {
		t.Fatalf("verify failed %v %+v", err, s)
	}
	if _, err := NewIssuer("other", time.Hour).Verify(tok); err == nil {
		t.Fatal("wrong secret must fail")
	}
}
