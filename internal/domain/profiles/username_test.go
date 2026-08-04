package profiles_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/merlindorin/sshark-api/internal/domain/profiles"
)

func TestValidateUsername(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		username string
		wantErr  error
	}{
		"a plain handle":            {username: "merlindorin"},
		"the shortest allowed":      {username: "abc"},
		"digits and dashes":         {username: "user-42"},
		"underscores in the middle": {username: "some_user"},
		"mixed case is kept":        {username: "MerlinDorin"},
		"the longest allowed":       {username: strings.Repeat("a", profiles.MaxUsernameLength)},
		"too short":                 {username: "ab", wantErr: profiles.ErrUsernameInvalid},
		"too long": {
			username: strings.Repeat("a", profiles.MaxUsernameLength+1),
			wantErr:  profiles.ErrUsernameInvalid,
		},
		"empty":                        {username: "", wantErr: profiles.ErrUsernameInvalid},
		"leading dash reads as a flag": {username: "-merlin", wantErr: profiles.ErrUsernameInvalid},
		"trailing dash":                {username: "merlin-", wantErr: profiles.ErrUsernameInvalid},
		"a dot would split the route":  {username: "merlin.keys", wantErr: profiles.ErrUsernameInvalid},
		"a slash would add a segment":  {username: "merlin/keys", wantErr: profiles.ErrUsernameInvalid},
		"an at sign is the prefix":     {username: "@merlin", wantErr: profiles.ErrUsernameInvalid},
		"spaces":                       {username: "merlin dorin", wantErr: profiles.ErrUsernameInvalid},
		"a reserved route":             {username: "settings", wantErr: profiles.ErrUsernameReserved},
		"a reserved route, any case":   {username: "Explore", wantErr: profiles.ErrUsernameReserved},
		"the removed sources route":    {username: "sources", wantErr: profiles.ErrUsernameReserved},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := profiles.ValidateUsername(test.username)

			if test.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateUsername(%q) = %v, want nil", test.username, err)
				}
				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Errorf("ValidateUsername(%q) = %v, want %v", test.username, err, test.wantErr)
			}
		})
	}
}
