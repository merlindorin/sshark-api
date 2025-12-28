package redisquery_test

import (
	"testing"

	"github.com/merlindorin/sshark-api/internal/redisquery"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Full-text search
		{name: "simple term", input: "merlindorin"},
		{name: "multiple terms", input: "merlin dorin"},
		{name: "wildcard", input: "merlin*"},

		// TEXT field search
		{name: "text field", input: "@username:merlindorin"},
		{name: "text field wildcard", input: "@username:merlin*"},
		{name: "text field id", input: "@id:f2af7412*"},
		{name: "text field key", input: "@key:AAAAC3NzaC1*"},

		// TAG field search
		{name: "tag field source", input: "@source:{github}"},
		{name: "tag field type", input: "@type:{ssh-ed25519}"},
		{name: "tag field OR", input: "@source:{github|gitlab}"},
		{name: "tag field multiple OR", input: "@source:{github|gitlab|bitbucket}"},

		// Exact phrase
		{name: "exact phrase", input: `"ssh-ed25519 AAAAC3"`},

		// Negation
		{name: "negation term", input: "-gitlab"},
		{name: "negation field", input: "-@source:{gitlab}"},

		// Optional
		{name: "optional term", input: "~backup"},

		// Fuzzy
		{name: "fuzzy 1", input: "%merln%"},
		{name: "fuzzy 2", input: "%%merln%%"},

		// OR
		{name: "or terms", input: "github|gitlab"},

		// Grouping
		{name: "grouped or", input: "(github|gitlab)"},
		{name: "grouped with field", input: "(@source:{github}|@source:{gitlab})"},

		// Combined
		{name: "text and tag", input: "@username:merlin* @source:{github}"},
		{name: "complex", input: "(@source:{github}|@source:{gitlab}) @username:merlin* -@type:{ssh-rsa}"},

		// Match all
		{name: "match all", input: "*"},

		// Any field name is valid (generic parser)
		{name: "any field", input: "@custom:value"},
		{name: "any tag field", input: "@custom:{value}"},

		// Error cases
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := redisquery.Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != nil {
				t.Logf("Parse(%q) = %s", tt.input, result.String())
			}
		})
	}
}
