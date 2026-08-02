package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSecretDosNotLeakInJSON(t *testing.T) {
	s := Secret("senha-real")

	out, err := json.Marshal(struct{ P Secret }{P: s})
	if err != nil {
		t.Fatalf("secret could not be marshaled: %v", err)
	}

	if strings.Contains(string(out), "senha-real") {
		t.Fatal("secret leaked in JSON")
	}
}

func TestSecretDosNotLeakInString(t *testing.T) {
	s := Secret("senha-real")
	if strings.Contains(s.String(), "senha-real") {
		t.Fatal("secret leaked in string: " + s.String())
	}

	if s.String() != "[REDACTED]" {
		t.Fatal("secret not redacted in string")
	}
}

func TestSecretRevealed(t *testing.T) {
	s := Secret("senha-real")

	if s.Reveal() != "senha-real" {
		t.Fatal("secret not revealed")
	}
}

func TestSecretDosNotLeakInLog(t *testing.T) {
	s := Secret("senha-real")
	if strings.Contains(s.LogValue().String(), "senha-real") {
		t.Fatal("secret leaked in log")
	}
}
