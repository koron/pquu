package gitapply

import (
	"fmt"
	"io"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// Parse parses a patch from r. It currently delegates to gitdiff.Parse but is
// intended to eventually use logic ported from Git's apply.c.
func Parse(r io.Reader) ([]*gitdiff.File, string, error) {
	return gitdiff.Parse(r)
}

// ApplyOptions defines options for patch application.
type ApplyOptions struct {
	// Reject allows applying a patch with rejects, writing failed hunks to a .rej file.
	Reject bool
}

// Apply applies the changes in f to src, writing the result to dst.
// It uses logic ported from Git's apply.c, supporting fuzz and context reduction.
func Apply(dst io.Writer, src io.ReaderAt, f *gitdiff.File, opts ApplyOptions) error {
	var srcBytes []byte
	if !f.IsNew {
		if src == nil {
			return fmt.Errorf("source is nil for non-new file %s", f.OldName)
		}
		// This is a bit inefficient for large files, but matches Git's behavior of loading into memory.
		// We need the whole file to support searching for hunk positions.
		// We limit the size to 1GB to avoid extreme memory usage, similar to Git's MAX_APPLY_SIZE.
		const maxApplySize = 1024 * 1024 * 1024
		var err error
		srcBytes, err = io.ReadAll(io.LimitReader(io.NewSectionReader(src, 0, maxApplySize+1), maxApplySize+1))
		if err != nil && err != io.EOF {
			return err
		}
		if len(srcBytes) > maxApplySize {
			return fmt.Errorf("file too large to apply patch")
		}
	}

	res, rejectedIndices, err := applyPatch(srcBytes, f, opts)
	if err != nil {
		return err
	}


	_, err = dst.Write(res)
	if err != nil {
		return err
	}

	if len(rejectedIndices) > 0 {
		return &RejectedError{Indices: rejectedIndices}
	}
	return nil
}

// RejectedError indicates that some hunks of the patch were rejected.
type RejectedError struct {
	Indices []int
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("patch applied with %d rejects", len(e.Indices))
}

// WriteRej writes the rejected hunks of f to w.
func WriteRej(w io.Writer, f *gitdiff.File, indices []int) error {
	fmt.Fprintf(w, "diff a/%s b/%s\t(rejected hunks)\n", f.NewName, f.NewName)
	for _, idx := range indices {
		frag := f.TextFragments[idx]
		_, err := io.WriteString(w, frag.String())
		if err != nil {
			return err
		}
		// Ensure newline
		s := frag.String()
		if len(s) > 0 && s[len(s)-1] != '\n' {
			_, err = io.WriteString(w, "\n")
			if err != nil {
				return err
			}
		}
	}
	return nil
}
