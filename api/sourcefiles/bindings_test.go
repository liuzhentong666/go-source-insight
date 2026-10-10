package sourcefiles

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"go-ai-study/api/fault"
)

func boundWorkspace(t *testing.T, host string, exclusions ...string) (*Workspace, string) {
	t.Helper()
	root := t.TempDir()
	w, err := NewWithBindings(t.TempDir(), []Binding{{OwnerID: 7, ProjectID: 1, HostRoot: host, ContainerRoot: root, ExcludePaths: exclusions}})
	if err != nil {
		t.Fatal(err)
	}
	return w, root
}

func TestBindingTargetMapping(t *testing.T) {
	w, root := boundWorkspace(t, `F:\Repository`, "sources", "volumes")
	if err := os.Mkdir(filepath.Join(root, "Pkg"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Pkg", "Code.go"), []byte("package example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for input, expected := range map[string]string{
		`F:\Repository`: ".", `f:/repository/`: ".", `f:\REPOSITORY\pkg\code.go`: "Pkg/Code.go", "F:/Repository/Pkg": "Pkg", "Pkg/Code.go": "Pkg/Code.go", "": ".",
	} {
		got, err := w.NormalizeTarget(7, 1, input)
		if err != nil || got != expected {
			t.Fatalf("mapping %q=%q %v", input, got, err)
		}
		if err := w.Check(7, 1, got); err != nil {
			t.Fatalf("mapped target unavailable: %v", err)
		}
	}
	for _, input := range []string{`F:\RepositorySibling`, `F:\Repository\..\Repository\Pkg`, `F:Repository\Pkg`, `\\server\share`, `\\?\F:\Repository`, `F:\Repository\Code.go:stream`, `F:\Repository\aux.go`, `F:\Repository\\Pkg`, `F:\Repository\.\Pkg`, `G:\Repository`, "/Repository", "../Pkg"} {
		if _, err := w.NormalizeTarget(7, 1, input); !errors.Is(err, fault.ErrInvalidTarget) {
			t.Fatalf("unsafe input %q accepted: %v", input, err)
		}
	}
	for _, input := range []string{`F:\Repository\SOURCES`, "sources/secret.go", "volumes"} {
		if _, err := w.NormalizeTarget(7, 1, input); !errors.Is(err, fault.ErrExcludedTarget) {
			t.Fatalf("excluded input %q accepted: %v", input, err)
		}
	}
	for _, identity := range [][2]uint{{8, 1}, {7, 2}} {
		if _, err := w.NormalizeTarget(identity[0], identity[1], `F:\Repository`); !errors.Is(err, fault.ErrInvalidTarget) {
			t.Fatalf("foreign binding selected: %v", err)
		}
	}
	posix, _ := boundWorkspace(t, "/repository/project")
	if got, err := posix.NormalizeTarget(7, 1, "/repository/project/pkg/code.go"); err != nil || got != "pkg/code.go" {
		t.Fatalf("POSIX mapping=%s %v", got, err)
	}
	for _, bad := range []string{"/Repository/project", "/repository/projects", "/repository/project/../project"} {
		if _, err := posix.NormalizeTarget(7, 1, bad); !errors.Is(err, fault.ErrInvalidTarget) {
			t.Fatalf("POSIX unsafe mapping: %v", err)
		}
	}
}

func TestBindingExclusionsAndRelativeSources(t *testing.T) {
	w, root := boundWorkspace(t, `F:\Repository`, "sources", "volumes/cache")
	for _, directory := range []string{"sources", "sourcesSibling", "business/sources", "volumes/cache", "volumes/code", ".runtime", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(directory), "code.go"), []byte("package example\n//line hidden.go:900\nfunc Hello(){}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := w.Scan(context.Background(), 7, 1, ".")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Files, ",") != "business/sources/code.go,sourcesSibling/code.go,volumes/code/code.go" {
		t.Fatalf("wrong exclusions: %+v", result)
	}
	for _, target := range []string{"sources", "sources/code.go", "volumes/cache/code.go", ".runtime/code.go", "bin/code.go"} {
		if err := w.Check(7, 1, target); !errors.Is(err, fault.ErrExcludedTarget) {
			t.Fatalf("direct check bypass=%v", err)
		}
		if _, err := w.Scan(context.Background(), 7, 1, target); !errors.Is(err, fault.ErrExcludedTarget) {
			t.Fatalf("direct scan bypass=%v", err)
		}
		if _, err := w.Analyze(context.Background(), 7, 1, target); !errors.Is(err, fault.ErrExcludedTarget) {
			t.Fatalf("direct analyze bypass=%v", err)
		}
		if err := w.WithFS(7, 1, func(files fs.FS) error { _, err := fs.ReadFile(files, target); return err }); !errors.Is(err, fault.ErrExcludedTarget) {
			t.Fatalf("direct index filesystem bypass=%v", err)
		}
	}
	info := w.Info(7, 1)
	encoded, _ := json.Marshal(info)
	if !info.Available || !info.AbsoluteInputAllowed || info.HostRoot != `F:\Repository` || info.BindingID == "" || strings.Contains(string(encoded), root) {
		t.Fatalf("invalid source info: %s", encoded)
	}
	foreign := w.Info(8, 1)
	if foreign.BindingConfigured || foreign.HostRoot != "" || foreign.Available {
		t.Fatalf("foreign info=%+v", foreign)
	}
}

func TestBindingMissingAndLinkRoots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	w, err := NewWithBindings(t.TempDir(), []Binding{{OwnerID: 7, ProjectID: 1, HostRoot: "/host/project", ContainerRoot: root}})
	if err != nil {
		t.Fatal(err)
	}
	if info := w.Info(7, 1); info.Available || info.HostRoot != "/host/project" || info.UnavailableCode != "source_unavailable" {
		t.Fatalf("missing info=%+v", info)
	}
	if _, err := w.Scan(context.Background(), 7, 1, "."); !errors.Is(err, fault.ErrSourceUnavailable) {
		t.Fatalf("missing root=%v", err)
	}
	makeDirectoryLink(t, root, t.TempDir())
	if _, err := w.Scan(context.Background(), 7, 1, "."); !errors.Is(err, fault.ErrUnsafePath) {
		t.Fatalf("linked binding root=%v", err)
	}
	if info := w.Info(7, 1); info.Available || info.UnavailableCode != "unsafe_path" {
		t.Fatalf("linked info=%+v", info)
	}
}

func TestBindingCaseAmbiguityRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filesystem cannot create case-distinct siblings")
	}
	w, root := boundWorkspace(t, `C:\Repository`)
	for _, file := range []string{"Code.go", "code.go"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("package p"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.NormalizeTarget(7, 1, `c:\repository\CODE.go`); !errors.Is(err, fault.ErrUnsafePath) {
		t.Fatalf("ambiguous casing allowed: %v", err)
	}
}

func TestCaseInsensitiveDirectoryEntryRejectsAmbiguityOnEveryOS(t *testing.T) {
	entries, err := fs.ReadDir(fstest.MapFS{"Code.go": {Data: []byte("package p")}, "code.go": {Data: []byte("package p")}}, ".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := caseInsensitiveEntry(entries, "CODE.go"); !errors.Is(err, fault.ErrUnsafePath) {
		t.Fatalf("case-distinct entries accepted: %v", err)
	}
	if match, err := caseInsensitiveEntry(entries[:1], "CODE.go"); err != nil || match == nil || match.Name() != "Code.go" {
		t.Fatalf("unique actual spelling not preserved: %v %v", match, err)
	}
}

func TestDirectoryFilteringUsesBindingCasePolicy(t *testing.T) {
	windows, _ := boundWorkspace(t, `F:\Repository`, "sources")
	if !windows.files(nil, 7, 1).filter(".RUNTIME", true) && runtime.GOOS != "windows" {
		t.Fatal("Windows host case variant was not excluded before walking")
	}
	if !windows.excluded(7, 1, "SOURCES/secret.go", false) {
		t.Fatal("Windows exclusion comparison must ignore case")
	}
	posix, _ := boundWorkspace(t, "/host/project", "sources")
	if runtime.GOOS != "windows" {
		if posix.files(nil, 7, 1).filter(".RUNTIME", true) || posix.excluded(7, 1, "SOURCES/secret.go", false) {
			t.Fatal("POSIX case-sensitive directories unexpectedly excluded")
		}
	}
}

func TestBindingConfigurationValidation(t *testing.T) {
	valid := Binding{OwnerID: 7, ProjectID: 1, HostRoot: `C:\Repository`, ContainerRoot: t.TempDir()}
	for _, change := range []func(*Binding){
		func(b *Binding) { b.OwnerID = 0 }, func(b *Binding) { b.ProjectID = 0 }, func(b *Binding) { b.HostRoot = `C:\` }, func(b *Binding) { b.HostRoot = "/" }, func(b *Binding) { b.HostRoot = "relative" }, func(b *Binding) { b.HostRoot = `C:\Repository\..\Elsewhere` }, func(b *Binding) { b.ContainerRoot = "relative" }, func(b *Binding) { b.ContainerRoot = filepath.VolumeName(b.ContainerRoot) + string(filepath.Separator) }, func(b *Binding) { b.ExcludePaths = []string{"."} }, func(b *Binding) { b.ExcludePaths = []string{"../sources"} }, func(b *Binding) { b.ExcludePaths = []string{"sources", "SOURCES"} },
	} {
		candidate := valid
		change(&candidate)
		if _, err := NewWithBindings(t.TempDir(), []Binding{candidate}); err == nil {
			t.Fatalf("invalid binding accepted: %+v", candidate)
		}
	}
	if _, err := NewWithBindings(t.TempDir(), []Binding{valid, valid}); err == nil {
		t.Fatal("duplicate binding accepted")
	}
	file := filepath.Join(t.TempDir(), "bindings.json")
	for _, text := range []string{`null`, `{}`, `{"bindings":null}`, `{"bindings":[{"owner_id":7,"project_id":1,"host_root":"/host/project","container_root":"/container/project","extra":true}]}`, `{} {}`, `{"bindings":false}`} {
		if err := os.WriteFile(file, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadBindings(file); err == nil {
			t.Fatalf("invalid JSON accepted")
		}
	}
}
