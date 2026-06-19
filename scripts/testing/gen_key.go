package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("PUBKEY=%s\n", base64.StdEncoding.EncodeToString(pub))
	fmt.Printf("PRIVKEY=%s\n", base64.StdEncoding.EncodeToString(priv))
}
