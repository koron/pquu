package main

import (
	"context"
	"fmt"

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

	series, err := loadSeries()
	if err != nil {
		return err
	}
	// TODO:
	for _, patch := range series {
		fmt.Printf("Applying patch..%s\n", patch)
		err := pushPatch(patch)
		if err != nil {
			return err
		}
	}

	return nil
}

func pushPatch(patch string) error {
	files, preamble, err := loadPatch(patch)
	if err != nil {
		return err
	}
	fmt.Println(patch)
	for i, f := range files {
		fmt.Printf("  #%d old=%s new=%s (isnew=%t isdel=%t iscopy=%t isren=%t)\n", i, f.OldName, f.NewName, f.IsNew, f.IsDelete, f.IsCopy, f.IsRename)
		// TODO: gitdiff.Apply()
	}
	// TODO: commit
	_ = preamble
	return nil
}
