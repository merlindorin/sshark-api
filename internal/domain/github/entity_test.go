package github_test

import (
	"testing"

	"github.com/merlindorin/sshark-api/internal/domain/github"
)

func TestSanitizeUsername(t *testing.T) {
	type args struct {
		username string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic",
			args: args{
				username: "merlin",
			},
			want: "merlin",
		},
		{
			name: "multiple",
			args: args{
				username: "merlin and dorin",
			},
			want: "merlin",
		},
		{
			name: "left most match",
			args: args{
				username: "m$rlin",
			},
			want: "m",
		},
		{
			name: "accept dash in the middle",
			args: args{
				username: "m-rlin",
			},
			want: "m-rlin",
		},
		{
			name: "no dash at the end",
			args: args{
				username: "merlin-",
			},
			want: "merlin",
		},
		{
			name: "no dash at the beggining",
			args: args{
				username: "-merlin",
			},
			want: "merlin",
		},
		{
			name: "sanitizable chat",
			args: args{
				username: "!@#$%^&*()-_[]\\{}|<>,.?/\"'~`'",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := github.Sanitize(tt.args.username); got != tt.want {
				t.Errorf("SanitizeUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}
