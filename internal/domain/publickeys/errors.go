package publickeys

import "errors"

var (
	ErrKeyNotFound      = errors.New("public key not found")
	ErrKeyAlreadyExists = errors.New("public key already exists")
)
