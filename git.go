package main

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/koron/pquu/internal/gitapply"
)

var getRepo = sync.OnceValues(func() (*git.Repository, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
})

func getConfig() (*config.Config, error) {
	r, err := getRepo()
	if err != nil {
		return nil, err
	}
	return r.Config()
}

func getWorktree() (*git.Worktree, error) {
	r, err := getRepo()
	if err != nil {
		return nil, err
	}
	return r.Worktree()
}

func getWorktreeRoot() (string, error) {
	wt, err := getWorktree()
	if err != nil {
		return "", err
	}
	return wt.Filesystem.Root(), nil
}

var ErrNotFilesytemStorage = errors.New("not filesystem storage")

func getDotGit() (string, error) {
	r, err := getRepo()
	if err != nil {
		return "", err
	}
	st, ok := r.Storer.(*filesystem.Storage)
	if !ok {
		return "", ErrNotFilesytemStorage
	}
	return st.Filesystem().Root(), nil
}

var ErrNotConfigString = errors.New("not found config string")

func getConfigString(name, key string) (string, error) {
	c, err := getConfig()
	if err != nil {
		return "", err
	}
	for _, section := range c.Raw.Sections {
		if section.Name != name {
			continue
		}
		for _, option := range section.Options {
			if option.Key != key {
				continue
			}
			return option.Value, nil
		}
	}
	return "", ErrNotConfigString
}

var patchesDir = sync.OnceValues(func() (string, error) {
	dir, err := getConfigString("guilt", "patchesdir")
	if err != nil {
		if !errors.Is(err, ErrNotConfigString) {
			return "", err
		}
		dot, err := getDotGit()
		if err != nil {
			return "", err
		}
		return filepath.Join(dot, "patches"), nil
	}
	root, err := getWorktreeRoot()
	return filepath.EvalSymlinks(filepath.Join(root, dir))
})

const branchPrefix = "pquu/"

func getCurrentBranch() (string, error) {
	r, err := getRepo()
	if err != nil {
		return "", err
	}
	h, err := r.Head()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(h.Name().Short(), branchPrefix), nil
}

func getPatchesPath(name string) (string, error) {
	dir, err := patchesDir()
	if err != nil {
		return "", err
	}
	branch, err := getCurrentBranch()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, branch, name), nil
}

func loadSeries() ([]string, error) {
	path, err := getPatchesPath("series")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var series []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		patch := sc.Text()
		if x := strings.IndexByte(patch, '#'); x >= 0 {
			patch = patch[:x]
		}
		patch = strings.TrimSpace(patch)
		if patch == "" {
			continue
		}
		series = append(series, patch)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// TODO: consider guard
	return series, nil
}

func loadApplied() ([]string, error) {
	path, err := getPatchesPath("status")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var applied []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		applied = append(applied, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func loadPatch(name string) (files []*gitdiff.File, preamble string, err error) {
	patchPath, err := getPatchesPath(name)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(patchPath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	return gitapply.Parse(f)
}
