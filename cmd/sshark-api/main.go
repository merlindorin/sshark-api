package main

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	c "github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/commands"
	"github.com/merlindorin/sshark-api/cmd/sshark-api/commands/fetch"
	"github.com/merlindorin/sshark-api/cmd/sshark-api/commands/scrape"
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
		MetricServer: &globals.MetricServer{},
		HTTPServer:   &globals.HTTPServer{},
		Postgres:     &globals.Postgres{},

		Serve:   &commands.Serve{},
		Migrate: &commands.Migrate{},
		Scrape:  &scrape.Scrape{},
		Fetch:   &fetch.Fetch{},
	}

	ctx := kong.Parse(
		&cli,
		kong.Name(name),
		kong.Description(description),
		kong.UsageOnError(),
		kong.Configuration(
			kongyaml.Loader,
			fmt.Sprintf("/etc/%s/config.yaml", name),
			fmt.Sprintf("~/.config/%s/config.yaml", name),
		),
	)

	ctx.BindTo(context.Background(), (*context.Context)(nil))
	ctx.FatalIfErrorf(ctx.Run(cli.Commons, cli.Postgres, cli.HTTPServer, cli.MetricServer))
}

type CMD struct {
	*c.Commons
	*globals.Postgres     `embed:"" prefix:"postgres-"`
	*globals.HTTPServer   `embed:"" prefix:"http-"`
	*globals.MetricServer `embed:"" prefix:"otel-"`

	Serve   *commands.Serve   `cmd:"" help:"Start the API server"`
	Migrate *commands.Migrate `cmd:"" help:"Run database migrations"`
	Scrape  *scrape.Scrape    `cmd:"" help:"Scrape SSH keys from providers"`
	Fetch   *fetch.Fetch      `cmd:"" help:""`
}
