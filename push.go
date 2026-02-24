package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/koron/pquu/internal/gitapply"
	"github.com/koron-go/subcmd"
)

var pushCommand = subcmd.DefineCommand("push", "apply patch in queue", push)

func push(ctx context.Context, args []string) error {
	var (
		force  bool
		all    bool
		num    int
		reject bool
	)
	fs := subcmd.FlagSet(ctx)
	fs.BoolVar(&force, "force", false, "Force patching")
	fs.BoolVar(&all, "all", false, "Apply all patches")
	fs.IntVar(&num, "num", 0, "Number of patches to apply")
	fs.BoolVar(&reject, "reject", false, "Apply patch with rejects")
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
		err := pushPatch(patch, gitapply.ApplyOptions{Reject: reject})
		if err != nil {
			return err
		}
	}

	return nil
}

func pushPatch(patch string, opts gitapply.ApplyOptions) error {
	files, preamble, err := loadPatch(patch)
	if err != nil {
		return err
	}
	wt, err := getWorktree()
	if err != nil {
		return err
	}
	root := wt.Filesystem.Root()

	fmt.Println(patch)
	for i, f := range files {
		fmt.Printf("  #%d old=%s new=%s (isnew=%t isdel=%t iscopy=%t isren=%t)\n", i, f.OldName, f.NewName, f.IsNew, f.IsDelete, f.IsCopy, f.IsRename)

		var src io.ReaderAt
		var file *os.File
		if !f.IsNew {
			path := filepath.Join(root, f.OldName)
			var err error
			file, err = os.Open(path)
			if err != nil {
				return err
			}
			src = file
		}

		var dst bytes.Buffer
		err = gitapply.Apply(&dst, src, f, opts)
		if err != nil {
			if rejErr, ok := err.(*gitapply.RejectedError); ok {
				fmt.Printf("  #%d applied with %d rejects\n", i, len(rejErr.Indices))
				rejPath := filepath.Join(root, f.NewName+".rej")
				rejFile, cerr := os.Create(rejPath)
				if cerr != nil {
					return cerr
				}
				err = gitapply.WriteRej(rejFile, f, rejErr.Indices)
				rejFile.Close()
				if err != nil {
					return err
				}
			} else {
				if file != nil {
					file.Close()
				}
				return err
			}
		}

		if f.IsDelete || f.IsRename {
			path := filepath.Join(root, f.OldName)
			err = os.Remove(path)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}

		if !f.IsDelete {
			path := filepath.Join(root, f.NewName)
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			mode := f.NewMode
			if mode == 0 {
				mode = 0644
			}
			err = os.WriteFile(path, dst.Bytes(), mode)
			if err != nil {
				if file != nil {
					file.Close()
				}
				return err
			}
		}
		if file != nil {
			file.Close()
		}
	}
	// TODO: commit
	_ = preamble
	return nil
}
