// Example: verifying a license in a consuming application.
//
// Build the license and keys first:
//
//	go run ./cmd/license-tool genkey -priv private.pem -pub public.pem
//	# write a template.json (see README), then:
//	go run ./cmd/license-tool sign -key private.pem -in template.json -out license.lic
//
// Then embed public.pem in your binary (go:embed) and verify at startup:
//
//	go run ./examples/license -pub public.pem -in license.lic
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/qoderwork/go-infra/licensing"
	"github.com/qoderwork/go-infra/licensing/machine"
)

func main() {
	pubPath := flag.String("pub", "", "embedded public key PEM")
	licPath := flag.String("in", "", "license .lic path")
	flag.Parse()

	pubPEM, err := os.ReadFile(*pubPath)
	if err != nil {
		fatal(err)
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
	verifier := licensing.NewVerifier(pub, licensing.CurrentVersion).
		WithFingerprint(machine.Fingerprint)

	lic, err := verifier.Verify(data)
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
