package sources

import "errors"

var (
	ErrSourceNotFound      = errors.New("source not found")
	ErrSourceAlreadyExists = errors.New("source already exists")
)
