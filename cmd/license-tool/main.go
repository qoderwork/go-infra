// Command license-tool issues and verifies offline software licenses.
//
// Subcommands:
//
//	genkey      generate an Ed25519 key pair (private.pem + public.pem)
//	sign        sign a license template JSON into a .lic envelope
//	verify      verify a .lic envelope against a public key
//	fingerprint print the host system-uuid machine code (dmidecode -s system-uuid)
//
// The private key stays with you (the issuer). Ship only the public key
// embedded in your application; it can verify but never forge licenses.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/qoderwork/go-infra/licensing"
	"github.com/qoderwork/go-infra/licensing/machine"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "genkey":
		cmdGenkey(os.Args[2:])
	case "sign":
		cmdSign(os.Args[2:])
	case "verify":
		cmdVerify(os.Args[2:])
	case "fingerprint":
		cmdFingerprint(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `license-tool - issue and verify offline licenses

Usage:
  license-tool genkey      [-priv private.pem] [-pub public.pem]
  license-tool sign        -key private.pem -in template.json [-out license.lic] [-version 1] [-system-uuid ABCD1234...] [-aes aes.key]
  license-tool verify      -pub public.pem -in license.lic [-version 1] [-aes aes.key]
  license-tool fingerprint
`)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func cmdGenkey(args []string) {
	fs := flag.NewFlagSet("genkey", flag.ExitOnError)
	privOut := fs.String("priv", "private.pem", "output private key path")
	pubOut := fs.String("pub", "public.pem", "output public key path")
	_ = fs.Parse(args)

	pub, priv, err := licensing.GenerateKey()
	if err != nil {
		fatal(err)
	}
	privPEM, err := licensing.EncodePrivateKeyPEM(priv)
	if err != nil {
		fatal(err)
	}
	pubPEM, err := licensing.EncodePublicKeyPEM(pub)
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*privOut, privPEM, 0o600); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*pubOut, pubPEM, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (keep secret) and %s (ship with your app)\n", *privOut, *pubOut)
}

func cmdSign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	key := fs.String("key", "", "private key PEM path")
	in := fs.String("in", "", "license template JSON path")
	out := fs.String("out", "license.lic", "output .lic path")
	version := fs.Int("version", licensing.CurrentVersion, "license version")
	sysUUID := fs.String("system-uuid", "", "bind license to this host system-uuid (dmidecode -s system-uuid, upper case, no dashes); overrides template")
	aesKeyPath := fs.String("aes", "", "AES-256 key file for encrypted output; produces .enc instead of .lic")
	_ = fs.Parse(args)
	if *key == "" || *in == "" {
		fatal(fmt.Errorf("sign requires -key and -in"))
	}

	privPEM, err := os.ReadFile(*key)
	if err != nil {
		fatal(err)
	}
	priv, err := licensing.DecodePrivateKeyPEM(privPEM)
	if err != nil {
		fatal(err)
	}
	tpl, err := os.ReadFile(*in)
	if err != nil {
		fatal(err)
	}
	var lic licensing.License
	if err := json.Unmarshal(tpl, &lic); err != nil {
		fatal(fmt.Errorf("parse template: %w", err))
	}
	if lic.IssuedAt.IsZero() {
		lic.IssuedAt = time.Now().UTC()
	}
	if *sysUUID != "" {
		fp, err := machine.FingerprintFromSystemUUID(*sysUUID)
		if err != nil {
			fatal(fmt.Errorf("system-uuid: %w", err))
		}
		lic.Machine = &licensing.MachineBinding{
			Fingerprint: fp,
		}
	}

	signer, err := licensing.NewSigner(priv, *version)
	if err != nil {
		fatal(err)
	}

	if *aesKeyPath != "" {
		aesKey, err := os.ReadFile(*aesKeyPath)
		if err != nil {
			fatal(fmt.Errorf("read AES key: %w", err))
		}
		enc, err := signer.SignEncrypted(&lic, aesKey)
		if err != nil {
			fatal(err)
		}
		outPath := *out
		if outPath == "license.lic" {
			outPath = "license.enc"
		}
		if err := licensing.SaveEncryptedEnvelope(outPath, enc); err != nil {
			fatal(err)
		}
		fmt.Printf("signed+encrypted -> %s (version %d)\n", outPath, enc.Version)
		return
	}

	env, err := signer.Sign(&lic)
	if err != nil {
		fatal(err)
	}
	if err := licensing.SaveEnvelope(*out, env); err != nil {
		fatal(err)
	}
	fmt.Printf("signed -> %s (version %d)\n", *out, env.Version)
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	pubPath := fs.String("pub", "", "public key PEM path")
	in := fs.String("in", "", "license .lic or .enc path")
	version := fs.Int("version", licensing.CurrentVersion, "license version to trust")
	aesKeyPath := fs.String("aes", "", "AES-256 key file (for encrypted licenses)")
	_ = fs.Parse(args)
	if *pubPath == "" || *in == "" {
		fatal(fmt.Errorf("verify requires -pub and -in"))
	}

	pubPEM, err := os.ReadFile(*pubPath)
	if err != nil {
		fatal(err)
	}
	pub, err := licensing.DecodePublicKeyPEM(pubPEM)
	if err != nil {
		fatal(err)
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		fatal(err)
	}

	v, err := licensing.NewVerifier(pub, *version)
	if err != nil {
		fatal(err)
	}
	v.WithFingerprint(machine.Fingerprint)

	var lic *licensing.License
	if *aesKeyPath != "" {
		aesKey, err := os.ReadFile(*aesKeyPath)
		if err != nil {
			fatal(fmt.Errorf("read AES key: %w", err))
		}
		lic, err = v.VerifyEncrypted(data, aesKey)
	} else {
		lic, err = v.Verify(data)
	}
	if err != nil {
		fmt.Printf("INVALID: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("VALID\n  product:   %s\n  subject:   %s\n  features:  %v\n  capacity:  %v\n  expiry:    %s\n",
		lic.Product, lic.Subject, lic.Features, lic.Capacity, lic.Expiry.Format(time.RFC3339))
}

func cmdFingerprint(args []string) {
	_ = flag.NewFlagSet("fingerprint", flag.ExitOnError).Parse(args)
	fp, err := machine.Fingerprint()
	if err != nil {
		fatal(err)
	}
	fmt.Println(fp)
}
