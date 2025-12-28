package main

import (
	_ "embed"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	c "github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/commands"
)

const (
	name        = "sshark"
	description = "SSH public key lookup API"
)

//nolint:gochecknoglobals // these global variables exist to be overridden during build
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
		Serve: &commands.Serve{},
	}

	ctx := kong.Parse(
		&cli,
		kong.Name(name),
		kong.Description(description),
		kong.UsageOnError(),
		kong.Configuration(kongyaml.Loader, "/etc/sshark/config.yaml", "~/.config/sshark/config.yaml"),
	)

	ctx.FatalIfErrorf(ctx.Run(cli.Commons))
}

type CMD struct {
	*c.Commons
	Serve *commands.Serve `cmd:"" help:"Start the API server"`
}
