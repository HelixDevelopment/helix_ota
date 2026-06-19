package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <privkey-base64> <file-to-sign>\n", os.Args[0])
		os.Exit(1)
	}
	privRaw, err := base64.StdEncoding.DecodeString(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode privkey: %v\n", err)
		os.Exit(1)
	}
	priv := ed25519.PrivateKey(privRaw)

	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read file: %v\n", err)
		os.Exit(1)
	}
	hash := sha256.Sum256(data)
	sig := ed25519.Sign(priv, hash[:])
	fmt.Print(base64.StdEncoding.EncodeToString(sig))
}
