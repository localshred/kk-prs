package main

import (
	"os"

	"github.com/localshred/kk-prs/cmd"
	"github.com/localshred/koolkit/pkg/plugin"
)

func main() {
	meta := plugin.Meta{
		Describe: "Show open PRs with statuses or review requests; watch PRs for changes",
		Requires: []string{"gh"},
	}
	if handled, code := plugin.Handle(os.Args[1:], meta, os.Stdout); handled {
		os.Exit(code)
	}
	os.Exit(cmd.Execute())
}
