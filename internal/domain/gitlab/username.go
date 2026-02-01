package gitlab

import (
	"regexp"
)

// usernameRegex defines GitLab username format: start with alphanumeric or underscore,
// followed by alphanumeric, dots, dashes, or underscores, 1-255 chars.
const usernameRegex = "[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,254}"

var (
	usernameRegexFind  = regexp.MustCompile(usernameRegex)
	usernameRegexValid = regexp.MustCompile("^" + usernameRegex + "$")
)

// Username represents a validated GitLab username.
type Username string

// String returns the username as a string.
func (u Username) String() string {
	return string(u)
}

// IsValid checks if the username matches GitLab's username format.
func (u Username) IsValid() bool {
	return IsValid(string(u))
}

// IsValid checks if a string is a valid GitLab username.
func IsValid(username string) bool {
	return usernameRegexValid.MatchString(username)
}

// Sanitize extracts a valid GitLab username from the input string.
func Sanitize(username string) string {
	return usernameRegexFind.FindString(username)
}
