package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"golang.org/x/crypto/curve25519"
)

func main() {
	// Generate random private key
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate private key: %v\n", err)
		os.Exit(1)
	}

	// Compute public key
	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to compute public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Private Key: %s\n", hex.EncodeToString(privateKey[:]))
	fmt.Printf("Public Key:  %s\n", hex.EncodeToString(publicKey))
}
