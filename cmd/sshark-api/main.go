package main

import (
	"context"
	_ "embed"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	c "github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/commands"
	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
)

const (
	name        = "sshark"
	description = "SSH public key lookup API"
)

//nolint:gochecknoglobals // these globals variables exist to be overridden during build
var (
	license string

	version     = "dev"
	commit      = "dirty"
	date        = "latest"
	buildSource = "source"
)

func main() {
	cli := CMD{
		Commons: &c.Commons{
			Version: c.NewVersion(name, version, commit, buildSource, date),
			Licence: c.NewLicence(license),
		},
		Redis: &globals.Redis{},
		Serve: &commands.Serve{},
	}

	ctx := kong.Parse(
		&cli,
		kong.Name(name),
		kong.Description(description),
		kong.UsageOnError(),
		kong.Configuration(kongyaml.Loader, "/etc/sshark/config.yaml", "~/.config/sshark/config.yaml"),
	)

	ctx.BindTo(context.Background(), (*context.Context)(nil))
	ctx.FatalIfErrorf(ctx.Run(cli.Commons, cli.Redis))
}

type CMD struct {
	*c.Commons
	*globals.Redis `embed:"" prefix:"redis-"`
	Serve          *commands.Serve   `cmd:"" help:"Start the API server"`
	Migrate        *commands.Migrate `cmd:"" help:"Migrate the database"`
}
