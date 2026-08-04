// Package sshparse parses SSH public keys in the OpenSSH authorized_keys format.
package sshparse

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Key holds the parts of an OpenSSH public key line.
type Key struct {
	Algorithm   string
	Comment     string
	Fingerprint string
}

// Parse splits an OpenSSH public key line and computes its SHA256 fingerprint.
// Lines that are not in the expected format yield a zero-valued Key.
func Parse(keyLine string) Key {
	parts := strings.Fields(keyLine)

	const minParts = 2
	if len(parts) < minParts {
		return Key{}
	}

	key := Key{Algorithm: parts[0]}
	if len(parts) > minParts {
		key.Comment = strings.Join(parts[minParts:], " ")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return key
	}

	hash := sha256.Sum256(decoded)
	key.Fingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])

	return key
}

// Fingerprint returns the SHA256 fingerprint of an OpenSSH public key line,
// or an empty string when the line cannot be parsed.
func Fingerprint(keyLine string) string {
	return Parse(keyLine).Fingerprint
}
