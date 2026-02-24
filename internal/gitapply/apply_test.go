package gitapply

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

func TestApply(t *testing.T) {
	src := "line1\nline2\nline3\n"
	patch := `diff --git a/file b/file
index 1234567..89abcdef 100644
--- a/file
+++ b/file
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "line1\nline2 modified\nline3\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}
}

func TestApplyAppend(t *testing.T) {
	src := "line1\n"
	patch := `diff --git a/file b/file
--- a/file
+++ b/file
@@ -1,1 +1,2 @@
 line1
+line2
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "line1\nline2\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}
}

func TestApplyNewFile(t *testing.T) {
	patch := `diff --git a/newfile b/newfile
new file mode 100644
index 0000000..89abcdef
--- /dev/null
+++ b/newfile
@@ -0,0 +1,1 @@
+New file content
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, nil, files[0], ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "New file content\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}
}

func TestApplyReject(t *testing.T) {
	src := "line1\nline2\nline3\n"
	patch := `diff --git a/file b/file
--- a/file
+++ b/file
@@ -1,2 +1,2 @@
-line1
+LINE1
 line2
@@ -3,1 +3,1 @@
-NOT MATCH
+MODIFIED
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], ApplyOptions{Reject: true})
	if err == nil {
		t.Fatal("Expected error (RejectedError), got nil")
	}

	rejErr, ok := err.(*RejectedError)
	if !ok {
		t.Fatalf("Expected *RejectedError, got %T", err)
	}

	if len(rejErr.Indices) != 1 || rejErr.Indices[0] != 1 {
		t.Errorf("Expected rejected index [1], got %v", rejErr.Indices)
	}

	expected := "LINE1\nline2\nline3\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}

	var rejBuf bytes.Buffer
	err = WriteRej(&rejBuf, files[0], rejErr.Indices)
	if err != nil {
		t.Fatalf("WriteRej failed: %v", err)
	}

	rejStr := rejBuf.String()
	if !strings.Contains(rejStr, "rejected hunks") {
		t.Errorf("Expected rejected hunks header, got:\n%s", rejStr)
	}
	if !strings.Contains(rejStr, "-NOT MATCH") {
		t.Errorf("Expected rejected hunk content, got:\n%s", rejStr)
	}
}

func TestApplyMultipleHunks(t *testing.T) {
	src := "line1\nline2\nline3\nline4\nline5\nline6\n"
	patch := `diff --git a/file b/file
--- a/file
+++ b/file
@@ -1,2 +1,2 @@
-line1
+LINE1
 line2
@@ -5,2 +5,2 @@
 line5
-line6
+LINE6
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "LINE1\nline2\nline3\nline4\nline5\nLINE6\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}
}

func TestApplyFuzz(t *testing.T) {
	// Shifted position
	src := "extra line\nline1\nline2\nline3\n"
	patch := `diff --git a/file b/file
--- a/file
+++ b/file
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "extra line\nline1\nline2 modified\nline3\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}
}

func TestApplyContextReduction(t *testing.T) {
	src := "line1 modified by someone\nline2\nline3\n"
	patch := `diff --git a/file b/file
--- a/file
+++ b/file
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`
	files, _, err := gitdiff.Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	expected := "line1 modified by someone\nline2 modified\nline3\n"
	if dst.String() != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, dst.String())
	}
}
