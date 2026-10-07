package db

import (
	"encoding/hex"
	"testing"
)

func TestKnownLocalMigration76IsExact(t *testing.T) {
	migrations, err := LoadEmbeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var m Migration
	for _, entry := range migrations {
		if entry.Version == 76 {
			m = entry
		}
	}
	raw, _ := hex.DecodeString("85d2ef79c1341c99d9c491a488990b5a6330f40977428c0ae95902e26f0085e5")
	var old [32]byte
	copy(old[:], raw)
	if !knownLocalMigration76(m, old) {
		t.Fatal("documented prior migration not recognized")
	}
	altered := m
	altered.Checksum[0] ^= 1
	if knownLocalMigration76(altered, old) {
		t.Fatal("arbitrary SQL change accepted")
	}
	old[0] ^= 1
	if knownLocalMigration76(m, old) {
		t.Fatal("arbitrary database checksum accepted")
	}
}
