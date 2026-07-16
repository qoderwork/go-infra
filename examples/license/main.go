// Example: verifying a license in a consuming application.
//
// Build keys and license:
//
//	go run ./cmd/license-tool genkey -priv private.pem -pub public.pem
//	# write a template.json (see README), then:
//	go run ./cmd/license-tool sign -key private.pem -in template.json -out license.lic
//
// For encrypted licenses:
//
//	go run ./cmd/license-tool sign -key private.pem -aes aes.key -in template.json -out license.enc
//
// Then embed public.pem (and optionally aes.key) in your binary via go:embed:
//
//	//go:embed public.pem
//	var embeddedPubKey []byte
//
// Verify at startup:
//
//	go run ./examples/license -in license.lic
//	go run ./examples/license -in license.enc -aes aes.key
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"github.com/qoderwork/go-infra/licensing"
	"github.com/qoderwork/go-infra/licensing/machine"
)

//go:embed testdata/public.pem
var embeddedPubKey []byte

func main() {
	licPath := flag.String("in", "", "license .lic or .enc path")
	aesKeyPath := flag.String("aes", "", "AES key file (for encrypted licenses)")
	pubPath := flag.String("pub", "", "public key PEM file (overrides embedded key)")
	flag.Parse()

	// Use embedded key by default, or load from file if specified.
	pubPEM := embeddedPubKey
	if *pubPath != "" {
		var err error
		pubPEM, err = os.ReadFile(*pubPath)
		if err != nil {
			fatal(err)
		}
	}

	pub, err := licensing.DecodePublicKeyPEM(pubPEM)
	if err != nil {
		fatal(err)
	}

	data, err := os.ReadFile(*licPath)
	if err != nil {
		fatal(err)
	}

	// Bind verification to this machine.
	verifier, err := licensing.NewVerifier(pub, licensing.CurrentVersion)
	if err != nil {
		fatal(err)
	}
	verifier.WithFingerprint(machine.Fingerprint)

	var lic *licensing.License

	if *aesKeyPath != "" {
		// Encrypted license: load AES key, then decrypt + verify.
		aesKey, err := os.ReadFile(*aesKeyPath)
		if err != nil {
			fatal(fmt.Errorf("read AES key: %w", err))
		}
		lic, err = verifier.VerifyEncrypted(data, aesKey)
	} else {
		// Plain signed license.
		lic, err = verifier.Verify(data)
	}
	if err != nil {
		fmt.Printf("license invalid: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("License OK for %s\n", lic.Subject)
	fmt.Printf("  features: %v\n", lic.Features)
	fmt.Printf("  capacity: %v\n", lic.Capacity)
	if lic.HasFeature("advanced") {
		fmt.Println("  -> advanced module enabled")
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
