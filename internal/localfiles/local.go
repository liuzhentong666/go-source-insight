// Package localfiles provides read-only, per-target host source access.
package localfiles

import (
	"context"
	"errors"
	"go-ai-study/internal/source"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafe = errors.New("unsafe source path")
var ErrTarget = errors.New("expected an absolute Go file or directory")

const MaxFileSize = 16 << 20

func IsAbsolute(name string) bool {
	return filepath.IsAbs(name) || (len(name) > 2 && name[1] == ':' && (name[2] == '\\' || name[2] == '/')) || strings.HasPrefix(name, `\\`)
}

// Open walks each ancestor through os.Root and rejects links/reparse points.
// A file target exposes exactly that file, never its siblings.
func Open(target string) (fs.FS, string, func(), error) {
	if !filepath.IsAbs(target) || strings.ContainsRune(target, 0) {
		return nil, "", nil, ErrTarget
	}
	target = filepath.Clean(target)
	anchor := filepath.VolumeName(target) + string(filepath.Separator)
	root, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, "", nil, err
	}
	parts := strings.Split(strings.TrimPrefix(target, anchor), string(filepath.Separator))
	single := ""
	for i, part := range parts {
		if part == "" {
			continue
		}
		info, e := root.Lstat(part)
		if e != nil {
			root.Close()
			return nil, "", nil, e
		}
		if unsafe(info) || (!info.IsDir() && !info.Mode().IsRegular()) {
			root.Close()
			return nil, "", nil, ErrUnsafe
		}
		if info.IsDir() && source.ShouldSkipDir(part) {
			root.Close()
			return nil, "", nil, fs.ErrPermission
		}
		if !info.IsDir() {
			if i != len(parts)-1 || !source.IsGoFile(part) {
				root.Close()
				return nil, "", nil, ErrTarget
			}
			single = part
			break
		}
		next, e := root.OpenRoot(part)
		if e != nil {
			root.Close()
			return nil, "", nil, e
		}
		after, e := next.Stat(".")
		root.Close()
		if e != nil || !os.SameFile(info, after) {
			next.Close()
			return nil, "", nil, ErrUnsafe
		}
		root = next
	}
	relative := "."
	if single != "" {
		relative = single
	}
	return &tree{root: root, single: single}, relative, func() { root.Close() }, nil
}

type tree struct {
	root   *os.Root
	single string
}

func (t *tree) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) || (t.single != "" && name != t.single) {
		return nil, fs.ErrPermission
	}
	parts := strings.Split(name, "/")
	parent := t.root
	defer func() {
		if parent != t.root {
			parent.Close()
		}
	}()
	for _, part := range parts[:len(parts)-1] {
		info, err := parent.Lstat(part)
		if err != nil {
			return nil, err
		}
		if unsafe(info) || !info.IsDir() {
			return nil, ErrUnsafe
		}
		next, err := parent.OpenRoot(part)
		if err != nil {
			return nil, err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(info, after) {
			next.Close()
			return nil, ErrUnsafe
		}
		if parent != t.root {
			parent.Close()
		}
		parent = next
	}
	leaf := parts[len(parts)-1]
	before, err := parent.Lstat(leaf)
	if err != nil {
		return nil, err
	}
	if unsafe(before) || (!before.IsDir() && !before.Mode().IsRegular()) {
		return nil, ErrUnsafe
	}
	if !before.IsDir() && (!source.IsGoFile(leaf) || before.Size() > MaxFileSize) {
		return nil, fs.ErrPermission
	}
	f, err := parent.Open(leaf)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		f.Close()
		return nil, ErrUnsafe
	}
	return f, nil
}

type Provider struct{ URL, Token string }

func FromEnvironment() (Provider, error) {
	p := Provider{URL: strings.TrimRight(os.Getenv("SOURCE_READER_URL"), "/"), Token: os.Getenv("SOURCE_READER_TOKEN")}
	if p.URL != "" && len(p.Token) < 32 {
		return p, errors.New("source reader requires a strong token")
	}
	return p, nil
}
func (p Provider) WithFS(ctx context.Context, target string, operation func(fs.FS, string) error) error {
	if p.URL != "" {
		relative, err := p.Resolve(ctx, target)
		if err != nil {
			return err
		}
		return operation(&remoteFS{ctx: ctx, provider: p, target: target}, relative)
	}
	files, relative, close, err := Open(target)
	if err != nil {
		return err
	}
	defer close()
	return operation(files, relative)
}
func (p Provider) Resolve(ctx context.Context, target string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p.URL != "" {
		var result wire
		if err := p.call(ctx, "/resolve", request{Target: target}, &result); err != nil {
			return "", err
		}
		return result.Target, nil
	}
	_, relative, close, err := Open(target)
	if err == nil {
		close()
	}
	return relative, err
}
