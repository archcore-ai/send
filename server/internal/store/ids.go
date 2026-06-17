package store

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"strings"
	"time"
)

// b32 is RFC 4648 base32 "extended hex" without padding: byte order is preserved
// lexicographically, so a time-prefixed id sorts by creation time.
var b32 = base32.HexEncoding.WithPadding(base32.NoPadding)

// NewSendID returns an unguessable, roughly time-sortable send id: "snd_" + a
// ULID-style 128-bit value (48-bit ms timestamp + 80 random bits).
func NewSendID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	mustRand(b[6:])
	return "snd_" + strings.ToLower(b32.EncodeToString(b[:]))
}

// NewRedeemToken returns a 192-bit random bearer token: "red_" + 48 hex chars.
// Only its HashToken is persisted (security-privacy R7).
func NewRedeemToken() string {
	var b [24]byte
	mustRand(b[:])
	return "red_" + hex.EncodeToString(b[:])
}

// mustRand fills b with cryptographic randomness; an entropy failure is
// unrecoverable for a security primitive, so it panics.
func mustRand(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("archcore-send: crypto/rand failed: " + err.Error())
	}
}
