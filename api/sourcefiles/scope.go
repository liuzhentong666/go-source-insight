package sourcefiles

import (
	"context"
	"io"
	"io/fs"
	"path"
	"strings"

	"go-ai-study/api/fault"
	core "go-ai-study/internal/analysis"
	"go-ai-study/internal/localfiles"
	"go-ai-study/internal/source"
)

// Scope retains the selected root, exclusions and canonical relative target.
// It owns no filesystem handles; each execution opens and closes its own root.
// Its private workspace is immutable after construction.
type Scope struct {
	workspace      *Workspace
	owner, project uint
	target         string
	hostTarget     string
	hostExcluded   []string
}

func (w *Workspace) localWorkspace(owner, project uint) *Workspace {
	binding := *w.local
	binding.OwnerID, binding.ProjectID = owner, project
	return &Workspace{root: w.root, analyzer: w.analyzer, bindings: map[bindingKey]Binding{{owner, project}: binding}}
}

func (w *Workspace) Resolve(owner, project uint, target string) (Scope, error) {
	return w.ResolveContext(context.Background(), owner, project, target)
}
func (w *Workspace) ResolveContext(ctx context.Context, owner, project uint, target string) (Scope, error) {
	if owner == 0 || project == 0 {
		return Scope{}, fault.ErrNotFound
	}
	selected := w
	if w.host != nil && localfiles.IsAbsolute(target) {
		clean, err := w.host.Resolve(ctx, target)
		if err != nil {
			return Scope{}, hostError(err)
		}
		excluded, err := w.hostExclusions(owner, project, target)
		if err != nil {
			return Scope{}, err
		}
		return Scope{workspace: w, owner: owner, project: project, target: clean, hostTarget: target, hostExcluded: excluded}, nil
	}
	// Relative targets always retain their explicit bound/legacy meaning.
	if w.local != nil && (strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\\:")) {
		selected = w.localWorkspace(owner, project)
	}
	clean, err := selected.NormalizeTarget(owner, project, target)
	if err != nil {
		return Scope{}, err
	}
	return Scope{workspace: selected, owner: owner, project: project, target: clean}, nil
}
func (s Scope) Target() string { return s.target }
func (s Scope) Check() error {
	return s.CheckContext(context.Background())
}
func (s Scope) CheckContext(ctx context.Context) error {
	if s.hostTarget != "" {
		_, err := s.workspace.host.Resolve(ctx, s.hostTarget)
		return hostError(err)
	}
	return s.workspace.Check(s.owner, s.project, s.target)
}
func (s Scope) Scan(ctx context.Context) (source.ScanResult, error) {
	if s.hostTarget != "" {
		var result source.ScanResult
		err := s.WithFSContext(ctx, func(files fs.FS) error { var err error; result, err = source.ScanFS(ctx, files, s.target); return err })
		return result, err
	}
	return s.workspace.Scan(ctx, s.owner, s.project, s.target)
}
func (s Scope) Analyze(ctx context.Context) (core.StaticAnalysisResult, error) {
	if s.hostTarget != "" {
		var result core.StaticAnalysisResult
		err := s.WithFSContext(ctx, func(files fs.FS) error {
			var err error
			result, err = s.workspace.analyzer.AnalyzeFS(ctx, files, s.target)
			return err
		})
		return result, err
	}
	return s.workspace.Analyze(ctx, s.owner, s.project, s.target)
}
func (s Scope) WithFS(operation func(fs.FS) error) error {
	return s.WithFSContext(context.Background(), operation)
}
func (s Scope) WithFSContext(ctx context.Context, operation func(fs.FS) error) error {
	if s.hostTarget != "" {
		return hostError(s.workspace.host.WithFS(ctx, s.hostTarget, func(files fs.FS, relative string) error {
			if relative != s.target {
				return fault.ErrSourceUnavailable
			}
			if len(s.hostExcluded) > 0 {
				_, windows, _ := absoluteComponents(s.hostTarget)
				files = &excludedFS{FS: files, paths: s.hostExcluded, windows: windows}
			}
			return operation(files)
		}))
	}
	return s.workspace.WithFS(s.owner, s.project, operation)
}

func (w *Workspace) hostExclusions(owner, project uint, target string) ([]string, error) {
	var candidates []Binding
	if w.local != nil {
		candidates = append(candidates, *w.local)
	}
	if binding, ok := w.binding(owner, project); ok {
		candidates = append(candidates, binding)
	}
	input, windows, err := absoluteComponents(target)
	if err != nil {
		return nil, fault.ErrInvalidTarget
	}
	for _, binding := range candidates {
		root, rootWindows, err := absoluteComponents(binding.HostRoot)
		if err != nil || windows != rootWindows || len(input) < len(root) {
			continue
		}
		matched := true
		for i := range root {
			if (windows && !strings.EqualFold(root[i], input[i])) || (!windows && root[i] != input[i]) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		relative := strings.Join(input[len(root):], "/")
		var result []string
		for _, exclude := range binding.ExcludePaths {
			candidate, blocked := relative, exclude
			if windows {
				candidate, blocked = strings.ToLower(candidate), strings.ToLower(blocked)
			}
			if candidate == blocked || strings.HasPrefix(candidate, blocked+"/") {
				return nil, fault.ErrExcludedTarget
			}
			if candidate == "" {
				result = append(result, exclude)
			} else if strings.HasPrefix(blocked, candidate+"/") {
				result = append(result, exclude[len(relative)+1:])
			}
		}
		return result, nil
	}
	return nil, nil
}

type excludedFS struct {
	fs.FS
	paths   []string
	windows bool
}

func (f *excludedFS) ShouldSkipSourceDir(name string) bool {
	if policy, ok := f.FS.(interface{ ShouldSkipSourceDir(string) bool }); ok {
		return policy.ShouldSkipSourceDir(name)
	}
	return source.ShouldSkipDir(name)
}

func (f *excludedFS) blocked(name string) bool {
	for _, exclude := range f.paths {
		candidate := name
		if f.windows {
			candidate, exclude = strings.ToLower(candidate), strings.ToLower(exclude)
		}
		if candidate == exclude || strings.HasPrefix(candidate, exclude+"/") {
			return true
		}
	}
	return false
}
func (f *excludedFS) Open(name string) (fs.File, error) {
	if f.blocked(name) {
		return nil, fault.ErrExcludedTarget
	}
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	if directory, ok := file.(fs.ReadDirFile); ok {
		return &excludedDirectory{File: file, directory: directory, filesystem: f, name: name}, nil
	}
	return file, nil
}

type excludedDirectory struct {
	fs.File
	directory  fs.ReadDirFile
	filesystem *excludedFS
	name       string
}

func (d *excludedDirectory) ReadDir(n int) ([]fs.DirEntry, error) {
	var result []fs.DirEntry
	for {
		count := n
		if n > 0 {
			count = n - len(result)
		}
		entries, err := d.directory.ReadDir(count)
		for _, entry := range entries {
			if !d.filesystem.blocked(path.Join(d.name, entry.Name())) {
				result = append(result, entry)
			}
		}
		if n <= 0 || len(result) == n || err != nil {
			if err == io.EOF && len(result) > 0 {
				err = nil
			}
			return result, err
		}
	}
}
