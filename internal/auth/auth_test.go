package auth

import (
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("hunter2hunter2")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "hunter2hunter2" {
		t.Fatal("password was stored in plain text")
	}
	if !CheckPassword(hash, "hunter2hunter2") {
		t.Fatal("correct password rejected")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("wrong password accepted")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	token, err := NewToken("user-123", "Ada", "admin", secret, time.Hour)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}

	claims, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-123" || claims.Name != "Ada" || claims.Role != "admin" {
		t.Fatalf("claims do not round-trip: %+v", claims)
	}

	if _, err := ParseToken(token, []byte("other-secret")); err == nil {
		t.Fatal("token signed with a different secret was accepted")
	}
	if _, err := ParseToken(token+"tampered", secret); err == nil {
		t.Fatal("tampered token was accepted")
	}
	if _, err := ParseToken("garbage", secret); err == nil {
		t.Fatal("garbage token was accepted")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	secret := []byte("test-secret")
	token, err := NewToken("user-123", "Ada", "user", secret, -time.Minute)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if _, err := ParseToken(token, secret); err == nil {
		t.Fatal("expired token was accepted")
	}
}
