package main

import (
	"context"

	"github.com/koron-go/subcmd"
)

var pushCommand = subcmd.DefineCommand("push", "apply patch in queue", push)

func push(ctx context.Context, args []string) error {
	var (
		force bool
		all   bool
		num   int
	)
	fs := subcmd.FlagSet(ctx)
	fs.BoolVar(&force, "force", false, "Force patching")
	fs.BoolVar(&all, "all", false, "Apply all patches")
	fs.IntVar(&num, "num", 0, "Number of patches to apply")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// TODO:

	return nil
}
