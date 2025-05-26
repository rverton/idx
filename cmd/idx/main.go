package main

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

var rootFlags = ff.NewFlagSet("idx")

var (
	useJSON = rootFlags.Bool('j', "json", "output in JSON format")
)

func main() {
	rootCmd := &ff.Command{
		Name:  "idx",
		Usage: "idx [FLAGS] <subcommand>",
		Flags: rootFlags,
		Subcommands: []*ff.Command{
			configCmd(),
		},
	}

	if err := rootCmd.ParseAndRun(
		context.Background(),
		os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(rootCmd))
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(0)
	}
}
