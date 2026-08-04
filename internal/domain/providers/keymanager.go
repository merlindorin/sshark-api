// Package providers exposes the operations sshark performs on a user's own account at a
// key provider, acting on their behalf with a delegated credential.
package providers

import (
	"context"

	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
)

// UserKey is a key owned by the authenticated user at a provider.
type UserKey struct {
	// ID is the identifier the provider uses to address the key.
	ID string
	// Fingerprint identifies the key material, and lets sshark match a stored key to its
	// provider counterpart when the provider identifier was never recorded.
	Fingerprint string
}

// KeyManager reads and removes the keys of the user whose credential it was built with.
type KeyManager interface {
	// ListKeys lists the authenticated user's keys of the given type.
	ListKeys(ctx context.Context, keyType publickeys.KeyType) ([]UserKey, error)

	// DeleteKey removes one of the authenticated user's keys.
	// It returns ErrKeyGone when the provider does not know the key anymore.
	DeleteKey(ctx context.Context, keyType publickeys.KeyType, keyID string) error
}
