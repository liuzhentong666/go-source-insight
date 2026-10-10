package sourcefiles

import (
	"context"
	"io/fs"
	"strings"

	"go-ai-study/api/fault"
	core "go-ai-study/internal/analysis"
	"go-ai-study/internal/source"
)

// Scope retains the selected root, exclusions and canonical relative target.
// It owns no filesystem handles; each execution opens and closes its own root.
// Its private workspace is immutable after construction.
type Scope struct {
	workspace      *Workspace
	owner, project uint
	target         string
}

func (w *Workspace) localWorkspace(owner, project uint) *Workspace {
	binding := *w.local
	binding.OwnerID, binding.ProjectID = owner, project
	return &Workspace{root: w.root, analyzer: w.analyzer, bindings: map[bindingKey]Binding{{owner, project}: binding}}
}

func (w *Workspace) Resolve(owner, project uint, target string) (Scope, error) {
	if owner == 0 || project == 0 {
		return Scope{}, fault.ErrNotFound
	}
	selected := w
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
func (s Scope) Check() error   { return s.workspace.Check(s.owner, s.project, s.target) }
func (s Scope) Scan(ctx context.Context) (source.ScanResult, error) {
	return s.workspace.Scan(ctx, s.owner, s.project, s.target)
}
func (s Scope) Analyze(ctx context.Context) (core.StaticAnalysisResult, error) {
	return s.workspace.Analyze(ctx, s.owner, s.project, s.target)
}
func (s Scope) WithFS(operation func(fs.FS) error) error {
	return s.workspace.WithFS(s.owner, s.project, operation)
}
