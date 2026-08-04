// Package identity resolves the external accounts a signed-in user has connected, so sshark
// can tell which provider accounts they own and act on those accounts on their behalf.
package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"

	"github.com/merlindorin/sshark-api/internal/domain/scraper"
)

// ErrNotConnected is returned when the user has no verified account at the provider.
var ErrNotConnected = errors.New("provider account not connected")

// clerkProviders maps a key provider to the Clerk OAuth strategy that proves ownership of an
// account at that provider.
func clerkProviders() map[scraper.Provider]string {
	return map[scraper.Provider]string{
		scraper.ProviderGitHub: "oauth_github",
		scraper.ProviderGitLab: "oauth_gitlab",
	}
}

// Account is a provider account the user has proven they own.
type Account struct {
	Provider scraper.Provider
	// Username is the login at the provider, which is how sshark indexes public keys.
	Username string
	// UserID is the provider's own identifier for the account.
	UserID string
}

// Resolver reads connected accounts and delegated credentials from Clerk.
type Resolver struct{}

// NewResolver creates a Clerk-backed identity resolver.
func NewResolver() *Resolver {
	return &Resolver{}
}

// Details is what sshark can show about a user, gathered in one Clerk call.
type Details struct {
	// DisplayName is the person's name as their connected accounts report it. Empty when they
	// never gave one.
	DisplayName string
	AvatarURL   string
	// Accounts are the provider accounts the user proved they own.
	Accounts []Account
	// CreatedAt is when the sshark account was created, as a Unix timestamp in milliseconds.
	CreatedAt int64
}

// Details returns the user's display information and connected accounts.
func (r *Resolver) Details(ctx context.Context, userID string) (*Details, error) {
	usr, err := user.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch user: %w", err)
	}

	details := &Details{
		Accounts:  accountsOf(usr),
		CreatedAt: usr.CreatedAt,
	}

	name := strings.TrimSpace(strings.Join(nonEmpty(usr.FirstName, usr.LastName), " "))
	if name == "" && len(details.Accounts) > 0 {
		name = details.Accounts[0].Username
	}
	details.DisplayName = name

	if usr.ImageURL != nil {
		details.AvatarURL = *usr.ImageURL
	}

	return details, nil
}

// Accounts returns every provider account the user has connected and verified.
func (r *Resolver) Accounts(ctx context.Context, userID string) ([]Account, error) {
	details, err := r.Details(ctx, userID)
	if err != nil {
		return nil, err
	}

	return details.Accounts, nil
}

func nonEmpty(values ...*string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil && *value != "" {
			result = append(result, *value)
		}
	}

	return result
}

func accountsOf(usr *clerk.User) []Account {
	accounts := make([]Account, 0, len(usr.ExternalAccounts))

	for _, external := range usr.ExternalAccounts {
		provider, ok := providerFor(external.Provider)
		if !ok {
			continue
		}

		if external.Username == nil || *external.Username == "" {
			continue
		}

		if !isVerified(external) {
			continue
		}

		accounts = append(accounts, Account{
			Provider: provider,
			Username: *external.Username,
			UserID:   external.ProviderUserID,
		})
	}

	return accounts
}

// Account returns the user's connected account at the given provider.
// It returns ErrNotConnected when there is none.
func (r *Resolver) Account(ctx context.Context, userID string, provider scraper.Provider) (*Account, error) {
	accounts, err := r.Accounts(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := range accounts {
		if accounts[i].Provider == provider {
			return &accounts[i], nil
		}
	}

	return nil, ErrNotConnected
}

// Token returns the delegated OAuth access token Clerk stores for the user at the given
// provider, along with the scopes that token was granted.
func (r *Resolver) Token(
	ctx context.Context,
	userID string,
	provider scraper.Provider,
) (string, []string, error) {
	strategy, ok := clerkProviders()[provider]
	if !ok {
		return "", nil, ErrNotConnected
	}

	list, err := user.ListOAuthAccessTokens(ctx, &user.ListOAuthAccessTokensParams{
		ID:       userID,
		Provider: strategy,
	})
	if err != nil {
		return "", nil, fmt.Errorf("cannot fetch oauth access token: %w", err)
	}

	for _, accessToken := range list.OAuthAccessTokens {
		if accessToken.Token != "" {
			return accessToken.Token, accessToken.Scopes, nil
		}
	}

	return "", nil, ErrNotConnected
}

// HasScope reports whether a granted scope list covers the requested scope.
func HasScope(scopes []string, scope string) bool {
	return slices.Contains(scopes, scope)
}

func providerFor(clerkProvider string) (scraper.Provider, bool) {
	for provider, strategy := range clerkProviders() {
		if strategy == clerkProvider {
			return provider, true
		}
	}

	return "", false
}

func isVerified(external *clerk.ExternalAccount) bool {
	// Clerk only omits the verification object for accounts it could not verify.
	return external.Verification != nil && external.Verification.Status == "verified"
}
