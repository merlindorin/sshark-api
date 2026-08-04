package profiles

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// MinUsernameLength is short enough for real short handles without allowing single letters,
	// which are the ones most worth keeping back.
	MinUsernameLength = 3
	// MaxUsernameLength matches GitHub's limit, so any provider login can be carried over.
	MaxUsernameLength = 39
)

// usernamePattern allows letters, digits, dashes and underscores, and requires the first and
// last character to be alphanumeric so a username never reads as a flag or a file extension.
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{1,37}[a-zA-Z0-9]$`)

// reservedUsernames are names sshark keeps for itself. They mirror the paths the app serves
// plus the platform words people expect to be unavailable, so /@settings can never be a
// person and no future route is stolen out from under us.
//
// Kept in sync with app/lib/reserved-usernames.ts.
func reservedUsernames() map[string]struct{} {
	names := []string{
		"about", "admin", "administrator", "api", "app", "blog", "dashboard", "docs",
		"documentation", "explore", "help", "home", "login", "logout", "profile", "register",
		"roadmap", "root", "search", "settings", "signin", "signout", "signup", "support",
		"terms", "privacy", "contact", "legal", "security", "status", "pricing", "features",
		"team", "careers", "jobs", "news", "press", "media", "public", "static", "assets",
		"images", "css", "js", "fonts", "sitemap", "robots", "favicon", "sources", "users",
		"user", "me", "sshark", "keys", "gpg",
	}

	reserved := make(map[string]struct{}, len(names))
	for _, name := range names {
		reserved[name] = struct{}{}
	}

	return reserved
}

// ValidateUsername checks a username is well formed and not reserved. It does not check
// whether someone already holds it, which only the repository can answer.
func ValidateUsername(username string) error {
	trimmed := strings.TrimSpace(username)

	if len(trimmed) < MinUsernameLength || len(trimmed) > MaxUsernameLength {
		return fmt.Errorf("%w: must be between %d and %d characters",
			ErrUsernameInvalid, MinUsernameLength, MaxUsernameLength)
	}

	if !usernamePattern.MatchString(trimmed) {
		return fmt.Errorf(
			"%w: use letters, digits, dashes and underscores, starting and ending with a letter or digit",
			ErrUsernameInvalid,
		)
	}

	if _, ok := reservedUsernames()[strings.ToLower(trimmed)]; ok {
		return fmt.Errorf("%w: %q", ErrUsernameReserved, trimmed)
	}

	return nil
}

// NormalizeUsername trims a username for storage. Case is preserved for display; uniqueness is
// enforced case-insensitively by the database.
func NormalizeUsername(username string) string {
	return strings.TrimSpace(username)
}
