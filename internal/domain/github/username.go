package github

import (
	"regexp"
)

// usernameRegex defines GitHub username format: alphanumeric, may contain hyphens, 1-39 chars.
const usernameRegex = "[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?"

var (
	usernameRegexFind  = regexp.MustCompile(usernameRegex)
	usernameRegexValid = regexp.MustCompile("^" + usernameRegex + "$")
)

// Username represents a validated GitHub username.
type Username string

// String returns the username as a string.
func (u Username) String() string {
	return string(u)
}

// IsValid checks if the username matches GitHub's username format.
func (u Username) IsValid() bool {
	return IsValid(string(u))
}

// IsValid checks if a string is a valid GitHub username.
func IsValid(username string) bool {
	return usernameRegexValid.MatchString(username)
}

// Sanitize extracts a valid GitHub username from the input string.
func Sanitize(username string) string {
	return usernameRegexFind.FindString(username)
}
