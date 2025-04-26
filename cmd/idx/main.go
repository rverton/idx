package main

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"
)

func main() {
	rootFlags := ff.NewFlagSet("idx")
	rootCmd := &ff.Command{
		Name:  "idx",
		Usage: "idx [FLAGS] <SUBCOMMAND>",
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
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(0)
	}
}
