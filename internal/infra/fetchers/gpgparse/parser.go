package gpgparse

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// ParsedKey contains parsed GPG key metadata.
type ParsedKey struct {
	Fingerprint  string
	Algorithm    string
	KeyBits      *int
	ExpiresAt    *time.Time
	UserIDs      []string
	Capabilities []string
}

// Parse parses an armored PGP public key and extracts metadata.
func Parse(armoredKey []byte) (*ParsedKey, error) {
	block, err := armor.Decode(bytes.NewReader(armoredKey))
	if err != nil {
		return nil, fmt.Errorf("decoding armor: %w", err)
	}

	if block.Type != openpgp.PublicKeyType {
		return nil, fmt.Errorf("unexpected block type: %s", block.Type)
	}

	entities, readErr := openpgp.ReadKeyRing(block.Body)
	if readErr != nil {
		return nil, fmt.Errorf("reading keyring: %w", readErr)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities found in key")
	}

	entity := entities[0]
	pk := entity.PrimaryKey

	parsed := &ParsedKey{
		Fingerprint:  strings.ToUpper(fmt.Sprintf("%X", pk.Fingerprint)),
		Algorithm:    algorithmName(pk.PubKeyAlgo),
		Capabilities: make([]string, 0),
		UserIDs:      make([]string, 0),
	}

	// Get key bits
	if bits, bitsErr := pk.BitLength(); bitsErr == nil {
		bitLen := int(bits)
		parsed.KeyBits = &bitLen
	}

	// Extract user IDs
	for _, identity := range entity.Identities {
		if identity.UserId != nil {
			parsed.UserIDs = append(parsed.UserIDs, identity.UserId.Id)
		}
	}

	// Extract capabilities from self-signatures
	parsed.Capabilities = extractCapabilities(entity)

	// Extract expiration from primary key self-signature
	parsed.ExpiresAt = extractExpiration(entity)

	return parsed, nil
}

func algorithmName(algo packet.PublicKeyAlgorithm) string {
	switch algo {
	case packet.PubKeyAlgoRSA, packet.PubKeyAlgoRSAEncryptOnly, packet.PubKeyAlgoRSASignOnly:
		return "RSA"
	case packet.PubKeyAlgoDSA:
		return "DSA"
	case packet.PubKeyAlgoElGamal:
		return "ELGAMAL"
	case packet.PubKeyAlgoECDSA:
		return "ECDSA"
	case packet.PubKeyAlgoEdDSA:
		return "EDDSA"
	case packet.PubKeyAlgoECDH:
		return "ECDH"
	case packet.PubKeyAlgoX25519:
		return "X25519"
	case packet.PubKeyAlgoX448:
		return "X448"
	case packet.PubKeyAlgoEd25519:
		return "ED25519"
	case packet.PubKeyAlgoEd448:
		return "ED448"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", algo)
	}
}

func extractCapabilities(entity *openpgp.Entity) []string {
	caps := make([]string, 0, 4)

	// Check primary key capabilities from self-signature
	for _, identity := range entity.Identities {
		if identity.SelfSignature == nil {
			continue
		}

		sig := identity.SelfSignature
		if sig.FlagSign {
			caps = appendUnique(caps, "sign")
		}
		if sig.FlagCertify {
			caps = appendUnique(caps, "certify")
		}
		if sig.FlagEncryptCommunications {
			caps = appendUnique(caps, "encrypt_comms")
		}
		if sig.FlagEncryptStorage {
			caps = appendUnique(caps, "encrypt_storage")
		}
		break // Only need first identity's self-signature
	}

	// Check subkeys for additional capabilities
	for _, subkey := range entity.Subkeys {
		if subkey.Sig == nil {
			continue
		}

		sig := subkey.Sig
		if sig.FlagSign {
			caps = appendUnique(caps, "sign")
		}
		if sig.FlagCertify {
			caps = appendUnique(caps, "certify")
		}
		if sig.FlagEncryptCommunications {
			caps = appendUnique(caps, "encrypt_comms")
		}
		if sig.FlagEncryptStorage {
			caps = appendUnique(caps, "encrypt_storage")
		}
		if sig.FlagAuthenticate {
			caps = appendUnique(caps, "authenticate")
		}
	}

	return caps
}

func extractExpiration(entity *openpgp.Entity) *time.Time {
	// Check primary identity self-signature for key expiration
	for _, identity := range entity.Identities {
		if identity.SelfSignature == nil {
			continue
		}

		if identity.SelfSignature.KeyLifetimeSecs != nil {
			lifetime := time.Duration(*identity.SelfSignature.KeyLifetimeSecs) * time.Second
			expiry := entity.PrimaryKey.CreationTime.Add(lifetime)
			return &expiry
		}
		break
	}

	return nil
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
