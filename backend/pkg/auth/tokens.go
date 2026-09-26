package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Session is the game session carried by the client as a bearer token.
type Session struct {
	AccountID   string `json:"sub"`
	CharacterID string `json:"cid,omitempty"`
	TelegramID  int64  `json:"tg,omitempty"`
	jwt.RegisteredClaims
}

// Issuer signs and verifies session tokens.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

func (i *Issuer) Issue(s Session) (string, time.Time, error) {
	exp := time.Now().Add(i.ttl)
	s.Subject = s.AccountID
	s.IssuedAt = jwt.NewNumericDate(time.Now())
	s.ExpiresAt = jwt.NewNumericDate(exp)
	s.Issuer = "ommrpg"
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, s).SignedString(i.secret)
	return tok, exp, err
}

var ErrBadToken = errors.New("invalid session token")

func (i *Issuer) Verify(tok string) (Session, error) {
	var s Session
	_, err := jwt.ParseWithClaims(tok, &s, func(t *jwt.Token) (any, error) {
		return i.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("ommrpg"), jwt.WithExpirationRequired())
	if err != nil || s.AccountID == "" {
		return Session{}, ErrBadToken
	}
	return s, nil
}

// CentrifugoClaims is the connection token format understood by Centrifugo.
type CentrifugoClaims struct {
	Info     map[string]any `json:"info,omitempty"`
	Channels []string       `json:"channels,omitempty"`
	jwt.RegisteredClaims
}

// CentrifugoToken issues a connection JWT (HS256) for user sub. `channels`
// are server-side subscriptions (e.g. the personal notification channel).
func CentrifugoToken(secret, sub string, info map[string]any, channels []string, ttl time.Duration) (string, error) {
	c := CentrifugoClaims{Info: info, Channels: channels}
	c.Subject = sub
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(ttl))
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}
