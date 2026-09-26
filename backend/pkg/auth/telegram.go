// Package auth validates Telegram Mini App (WebApp) launch data and issues
// the game's own session tokens and Centrifugo connection tokens.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TelegramUser is the "user" field of WebApp initData.
type TelegramUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	PhotoURL     string `json:"photo_url,omitempty"`
	IsPremium    bool   `json:"is_premium,omitempty"`
}

// InitData is the validated launch payload.
type InitData struct {
	User       TelegramUser
	AuthDate   time.Time
	QueryID    string
	StartParam string
}

var (
	ErrInitDataInvalid = errors.New("telegram init data invalid")
	ErrInitDataExpired = errors.New("telegram init data expired")
)

// ValidateInitData checks the HMAC signature of Telegram.WebApp.initData as
// described in https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func ValidateInitData(raw, botToken string, maxAge time.Duration, now time.Time) (InitData, error) {
	vals, err := url.ParseQuery(raw)
	if err != nil || botToken == "" {
		return InitData{}, ErrInitDataInvalid
	}
	gotHash := vals.Get("hash")
	if gotHash == "" {
		return InitData{}, ErrInitDataInvalid
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(vals.Get(k))
	}
	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	want := hex.EncodeToString(hmacSHA256(secret, []byte(b.String())))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(gotHash))) {
		return InitData{}, ErrInitDataInvalid
	}
	ts, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return InitData{}, ErrInitDataInvalid
	}
	d := InitData{AuthDate: time.Unix(ts, 0), QueryID: vals.Get("query_id"), StartParam: vals.Get("start_param")}
	if maxAge > 0 && now.Sub(d.AuthDate) > maxAge {
		return InitData{}, ErrInitDataExpired
	}
	if err := json.Unmarshal([]byte(vals.Get("user")), &d.User); err != nil || d.User.ID == 0 {
		return InitData{}, ErrInitDataInvalid
	}
	return d, nil
}

// SignInitData builds a valid initData string (used by tests and dev tools).
func SignInitData(botToken string, user TelegramUser, authDate time.Time) string {
	u, _ := json.Marshal(user)
	vals := url.Values{}
	vals.Set("user", string(u))
	vals.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	vals.Set("query_id", "AAE-dev")
	keys := []string{"auth_date", "query_id", "user"}
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + vals.Get(k))
	}
	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	vals.Set("hash", hex.EncodeToString(hmacSHA256(secret, []byte(b.String()))))
	return vals.Encode()
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
