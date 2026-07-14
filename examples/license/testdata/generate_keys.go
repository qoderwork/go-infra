//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/qoderwork/go-infra/licensing"
)

func main() {
	pub, priv, err := licensing.GenerateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate keys: %v\n", err)
		os.Exit(1)
	}
	pubPEM, err := licensing.EncodePublicKeyPEM(pub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode public key: %v\n", err)
		os.Exit(1)
	}
	privPEM, err := licensing.EncodePrivateKeyPEM(priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode private key: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("testdata/public.pem", pubPEM, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write public.pem: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("testdata/private.pem", privPEM, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write private.pem: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Keys generated: testdata/public.pem, testdata/private.pem")
}
