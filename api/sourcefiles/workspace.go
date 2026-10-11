// Package sourcefiles binds a project to a confined, read-only source tree.
package sourcefiles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-ai-study/api/fault"
	core "go-ai-study/internal/analysis"
	"go-ai-study/internal/localfiles"
	"go-ai-study/internal/source"
)

type Workspace struct {
	root     string
	analyzer *core.StaticAnalyzer
	bindings map[bindingKey]Binding
	local    *Binding
	host     *localfiles.Provider
}

// EnableHostAccess leaves relative project bindings unchanged.
func (w *Workspace) EnableHostAccess(provider localfiles.Provider) { w.host = &provider }

func New(root string) (*Workspace, error) {
	return NewWithBindings(root, nil)
}

func NewWithBindings(root string, bindings []Binding) (*Workspace, error) {
	return NewWithConfig(root, bindings, nil)
}

func NewWithConfig(root string, bindings []Binding, localRoot *LocalRoot) (*Workspace, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("SOURCE_ROOT must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid SOURCE_ROOT")
	}
	validated, err := validateBindings(bindings)
	if err != nil {
		return nil, err
	}
	local, err := validateLocalRoot(localRoot)
	if err != nil {
		return nil, err
	}
	return &Workspace{root: filepath.Clean(abs), analyzer: core.NewStaticAnalyzer(nil), bindings: validated, local: local}, nil
}

// NormalizeTarget deliberately uses portable slash-separated relative paths.
// Reject Windows streams/device aliases even when the API runs on Linux.
func NormalizeTarget(target string) (string, error) {
	if target == "" {
		return ".", nil
	}
	if !fs.ValidPath(target) || strings.ContainsAny(target, "\\:\x00") || filepath.IsAbs(target) {
		return "", fault.ErrInvalidTarget
	}
	for _, component := range strings.Split(target, "/") {
		if component == "." {
			continue
		}
		if strings.TrimRight(component, ". ") != component {
			return "", fault.ErrInvalidTarget
		}
		base := strings.ToUpper(strings.Split(component, ".")[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" {
			return "", fault.ErrInvalidTarget
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return "", fault.ErrInvalidTarget
		}
	}
	return target, nil
}

func (w *Workspace) open(ownerID, projectID uint) (*os.Root, error) {
	if ownerID == 0 || projectID == 0 {
		return nil, fault.ErrNotFound
	}
	bound := filepath.Join(w.root, strconv.FormatUint(uint64(ownerID), 10), strconv.FormatUint(uint64(projectID), 10))
	if binding, ok := w.binding(ownerID, projectID); ok {
		bound = binding.ContainerRoot
	}
	volume := filepath.VolumeName(bound)
	anchor := volume + string(filepath.Separator)
	root, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, sourceError(err)
	}
	components := strings.Split(strings.TrimPrefix(bound, anchor), string(filepath.Separator))
	for _, component := range components {
		if component == "" {
			continue
		}
		next, err := openDirectory(root, component)
		root.Close()
		if err != nil {
			return nil, sourceError(err)
		}
		root = next
	}
	return root, nil
}

// Each directory is verified against the opened handle before descending.
// os.Root prevents a concurrent replacement from escaping the parent handle.
func openDirectory(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if isReparse(before) {
		return nil, fault.ErrUnsafePath
	}
	if !before.IsDir() {
		return nil, fault.ErrSourceUnavailable
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		child.Close()
		return nil, fault.ErrUnsafePath
	}
	return child, nil
}

// Check validates existing path bindings without reading source contents.
func (w *Workspace) Check(ownerID, projectID uint, target string) error {
	root, err := w.open(ownerID, projectID)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := w.files(root, ownerID, projectID).Open(target)
	if err != nil {
		return sourceError(err)
	}
	return file.Close()
}

func (w *Workspace) Scan(ctx context.Context, ownerID, projectID uint, target string) (source.ScanResult, error) {
	root, err := w.open(ownerID, projectID)
	if err != nil {
		return source.ScanResult{}, err
	}
	defer root.Close()
	result, err := source.ScanFS(ctx, w.files(root, ownerID, projectID), target)
	if err != nil {
		return source.ScanResult{}, sourceError(err)
	}
	return result, nil
}

func (w *Workspace) Analyze(ctx context.Context, ownerID, projectID uint, target string) (core.StaticAnalysisResult, error) {
	root, err := w.open(ownerID, projectID)
	if err != nil {
		return core.StaticAnalysisResult{}, err
	}
	defer root.Close()
	result, err := w.analyzer.AnalyzeFS(ctx, w.files(root, ownerID, projectID), target)
	if err != nil {
		if ctx.Err() != nil {
			return core.StaticAnalysisResult{}, ctx.Err()
		}
		var accessError *source.AccessError
		if errors.Is(err, fault.ErrUnsafePath) {
			return core.StaticAnalysisResult{}, fault.ErrUnsafePath
		}
		if errors.As(err, &accessError) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			return core.StaticAnalysisResult{}, sourceError(err)
		}
		return core.StaticAnalysisResult{}, fmt.Errorf("%w: %v", fault.ErrInvalidSource, err)
	}
	return result, nil
}

func sourceError(err error) error {
	if errors.Is(err, fault.ErrExcludedTarget) {
		return fault.ErrExcludedTarget
	}
	if errors.Is(err, fault.ErrUnsafePath) {
		return fault.ErrUnsafePath
	}
	return fmt.Errorf("%w: %v", fault.ErrSourceUnavailable, err)
}

type confinedFS struct {
	root    *os.Root
	exclude func(string, bool) bool
	filter  func(string, bool) bool
}

func (w *Workspace) files(root *os.Root, owner, project uint) *confinedFS {
	binding, bound := w.binding(owner, project)
	_, windowsHost, _ := absoluteComponents(binding.HostRoot)
	return &confinedFS{root: root, exclude: func(name string, dir bool) bool { return w.excluded(owner, project, name, dir) }, filter: func(name string, dir bool) bool {
		// Standard directories remain visible so the scanner records SkippedDirs.
		// Explicit exclusions must disappear before WalkDir attempts to open them.
		return w.explicitlyExcluded(owner, project, name) || (bound && windowsHost && dir && source.ShouldSkipDir(strings.ToLower(filepath.Base(name))) && !source.ShouldSkipDir(filepath.Base(name)))
	}}
}

// WithFS opens a worker-owned project handle for the full indexing operation.
// The callback must finish all filesystem work before returning.
func (w *Workspace) WithFS(ownerID, projectID uint, operation func(fs.FS) error) error {
	root, err := w.open(ownerID, projectID)
	if err != nil {
		return err
	}
	defer root.Close()
	return operation(w.files(root, ownerID, projectID))
}

func (s *confinedFS) Open(name string) (fs.File, error) {
	clean, err := NormalizeTarget(name)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(clean, "/")
	parent := s.root
	defer func() {
		if parent != s.root {
			parent.Close()
		}
	}()
	for _, part := range parts[:len(parts)-1] {
		next, err := openDirectory(parent, part)
		if err != nil {
			return nil, err
		}
		if parent != s.root {
			parent.Close()
		}
		parent = next
	}
	leaf := parts[len(parts)-1]
	before, err := parent.Lstat(leaf)
	if err != nil {
		return nil, err
	}
	if isReparse(before) || (!before.IsDir() && !before.Mode().IsRegular()) {
		return nil, fault.ErrUnsafePath
	}
	if s.exclude != nil && s.exclude(clean, before.IsDir()) {
		return nil, fault.ErrExcludedTarget
	}
	file, err := parent.Open(leaf)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		file.Close()
		return nil, fault.ErrUnsafePath
	}
	if before.IsDir() && s.filter != nil {
		return &filteredDirectory{File: file, name: clean, exclude: s.filter}, nil
	}
	return file, nil
}

// Filtering directory entries prevents WalkDir from trying to open excluded
// directories. Direct access is still rejected, including descendants.
type filteredDirectory struct {
	*os.File
	name    string
	exclude func(string, bool) bool
}

func (d *filteredDirectory) ReadDir(n int) ([]fs.DirEntry, error) {
	entries := []fs.DirEntry{}
	for {
		batchSize := n
		if n > 0 {
			batchSize = n - len(entries)
		}
		batch, err := d.File.ReadDir(batchSize)
		for _, entry := range batch {
			name := entry.Name()
			if d.name != "." {
				name = d.name + "/" + name
			}
			if !d.exclude(name, entry.IsDir()) {
				entries = append(entries, entry)
			}
		}
		if n <= 0 || len(entries) == n || err != nil {
			return entries, err
		}
	}
}
