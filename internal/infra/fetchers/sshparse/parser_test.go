package sshparse_test

import (
	"testing"

	"github.com/merlindorin/sshark-api/internal/infra/fetchers/sshparse"
)

const (
	ed25519Key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPJC5pFIIp0UNJJqlIzw9cuo9BVt6R1A/cfNJBBXGd0c merlin@sshark"
	//nolint:lll // an RSA public key does not wrap
	rsaKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDBPsA1EF5jCxcPNfyWGvkPsjszp4vRTw01zB3UyQqI3DfMwwjLd27ZBOmov+ib7oz4JiiIorLdUYd9+CTB5oMhCCxfITAClIPcaWLpQWomiw1V1X7Iz7giWQ9iy+S+AUIdLyVFMtc9APv0B2+RpV814sB6RWm+901NxadBW5GD9MUvtze/CUv7/CmPq8uqegvL8HIZGOSP6U+D+3ir6W7g7h1ld8bkRRwN5QzK2g7jqmy2NjM/a/kuKxVn/ht+AAw2gwc7wwPRm7f6ol17tvGSGMxpk364R4Fevxh4DZekfzX8P2y7IUogM4tC0wAZgHEWeCIdinaT1M48P9EJPPOb"
)

// TestParse checks the fingerprint matches what ssh-keygen -lf reports. Revoking a key relies
// on matching a stored fingerprint against the one computed from the provider's copy of the
// key, so both sides have to agree on this exact format.
func TestParse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		keyLine string
		want    sshparse.Key
	}{
		"ed25519 with a comment": {
			keyLine: ed25519Key,
			want: sshparse.Key{
				Algorithm:   "ssh-ed25519",
				Comment:     "merlin@sshark",
				Fingerprint: "SHA256:NkYlKh3pDi8qJmZG/ZLXt0n7o14apo7vojm6TAvHOhM",
			},
		},
		"rsa without a comment": {
			keyLine: rsaKey,
			want: sshparse.Key{
				Algorithm:   "ssh-rsa",
				Fingerprint: "SHA256:D5htjaFLo9yKs1WBC8JR03Pes6BxJOQB7WWqFQ6/kow",
			},
		},
		"comment containing spaces": {
			keyLine: ed25519Key + " on my laptop",
			want: sshparse.Key{
				Algorithm:   "ssh-ed25519",
				Comment:     "merlin@sshark on my laptop",
				Fingerprint: "SHA256:NkYlKh3pDi8qJmZG/ZLXt0n7o14apo7vojm6TAvHOhM",
			},
		},
		"key material that is not base64": {
			keyLine: "ssh-ed25519 not-base64 merlin@sshark",
			want: sshparse.Key{
				Algorithm: "ssh-ed25519",
				Comment:   "merlin@sshark",
			},
		},
		"not a key at all": {
			keyLine: "garbage",
			want:    sshparse.Key{},
		},
		"empty line": {
			keyLine: "",
			want:    sshparse.Key{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := sshparse.Parse(test.keyLine)
			if got != test.want {
				t.Errorf("Parse(%q) = %+v, want %+v", test.keyLine, got, test.want)
			}

			if fingerprint := sshparse.Fingerprint(test.keyLine); fingerprint != test.want.Fingerprint {
				t.Errorf("Fingerprint(%q) = %q, want %q", test.keyLine, fingerprint, test.want.Fingerprint)
			}
		})
	}
}
