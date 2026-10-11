package sourcefiles

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-ai-study/api/fault"
	"go-ai-study/internal/localfiles"
)

func TestHostScopesOutsideConfiguredRootRemainIndependent(t *testing.T) {
	legacy, local, external := t.TempDir(), t.TempDir(), t.TempDir()
	for _, root := range []string{local, external, filepath.Join(legacy, "7", "1")} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package sample\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(local, "private"), 0700)
	os.WriteFile(filepath.Join(local, "private", "hidden.go"), []byte("package sample"), 0600)
	w, err := NewWithConfig(legacy, nil, &LocalRoot{HostRoot: local, ContainerRoot: local, ExcludePaths: []string{"private"}})
	if err != nil {
		t.Fatal(err)
	}
	w.EnableHostAccess(localfiles.Provider{})
	var scopes []Scope
	for _, target := range []string{local, filepath.Join(external, "main.go"), "."} {
		scope, err := w.ResolveContext(context.Background(), 7, 1, target)
		if err != nil {
			t.Fatal(err)
		}
		scopes = append(scopes, scope)
	}
	for _, scope := range scopes {
		result, err := scope.Scan(context.Background())
		if err != nil || len(result.Files) != 1 || result.Files[0] != "main.go" {
			t.Fatalf("scope=%+v scan=%+v err=%v", scope, result, err)
		}
	}
	if runtime.GOOS != "windows" {
		os.MkdirAll(filepath.Join(local, "Private"), 0700)
		os.WriteFile(filepath.Join(local, "Private", "business.go"), []byte("package business"), 0600)
		result, err := scopes[0].Scan(context.Background())
		if err != nil || len(result.Files) != 2 {
			t.Fatalf("POSIX case-distinct business directory excluded: %+v %v", result, err)
		}
	}
	if _, err := w.Resolve(7, 1, filepath.Join(local, "private")); !errors.Is(err, fault.ErrExcludedTarget) {
		t.Fatalf("lost exclusion: %v", err)
	}
	info := w.Info(7, 1)
	if info.SourceMode != "host" || !info.Available || !info.AbsoluteInputAllowed || info.HostRoot != "" {
		t.Fatalf("host info: %+v", info)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.ResolveContext(ctx, 7, 1, external); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resolve: %v", err)
	}
}

func TestLocalScopePreservesRelativeBindingAndLegacyRoots(t *testing.T) {
	legacy, bound, local := t.TempDir(), t.TempDir(), t.TempDir()
	for root, name := range map[string]string{filepath.Join(legacy, "7", "2"): "legacy.go", bound: "bound.go", local: "local.go"} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte("package sample\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &LocalRoot{HostRoot: `F:\Local`, ContainerRoot: local, ExcludePaths: []string{"private"}}
	w, err := NewWithConfig(legacy, []Binding{{OwnerID: 7, ProjectID: 1, HostRoot: `F:\Bound`, ContainerRoot: bound}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Configuration supplied by the caller cannot mutate an admitted scope.
	cfg.ContainerRoot = bound
	cfg.ExcludePaths[0] = "local.go"
	for _, tc := range []struct {
		project      uint
		target, want string
	}{{1, ".", "bound.go"}, {2, ".", "legacy.go"}, {1, `f:\LOCAL`, "local.go"}, {2, `F:/Local`, "local.go"}} {
		scope, err := w.Resolve(7, tc.project, tc.target)
		if err != nil {
			t.Fatal(err)
		}
		got, err := scope.Scan(context.Background())
		if err != nil || strings.Join(got.Files, ",") != tc.want || got.TargetPath != "." {
			t.Fatalf("scope=%+v result=%+v err=%v", tc, got, err)
		}
	}
	if _, err := w.Resolve(7, 1, `F:/Bound`); !errors.Is(err, fault.ErrInvalidTarget) {
		t.Fatalf("local bypass via bound absolute: %v", err)
	}
	if _, err := w.Resolve(7, 2, `F:/Local/private`); !errors.Is(err, fault.ErrExcludedTarget) {
		t.Fatalf("local exclusion=%v", err)
	}
	a, b := w.Info(7, 1), w.Info(7, 2)
	if a.SourceMode != "local" || !a.Available || a.HostRoot != `F:\Local` || a.BindingID == "" || a.BindingID != b.BindingID {
		t.Fatalf("info=%+v %+v", a, b)
	}
	w2, err := NewWithConfig(legacy, nil, &LocalRoot{HostRoot: `F:\Local`, ContainerRoot: local, ExcludePaths: []string{"other"}})
	if err != nil {
		t.Fatal(err)
	}
	if w2.Info(7, 1).BindingID == a.BindingID {
		t.Fatal("changed local exclusion did not invalidate context")
	}
}

func TestMissingLocalScopeNeverFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "7", "1"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "7", "1", "legacy.go"), []byte("package p"), 0600); err != nil {
		t.Fatal(err)
	}
	w, err := NewWithConfig(root, nil, &LocalRoot{HostRoot: `F:\Local`, ContainerRoot: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := w.Resolve(7, 1, `F:/Local`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Scan(context.Background()); !errors.Is(err, fault.ErrSourceUnavailable) {
		t.Fatalf("fallback=%v", err)
	}
	info := w.Info(7, 1)
	if info.Available || info.SourceMode != "local" || info.HostRoot != `F:\Local` {
		t.Fatalf("missing info=%+v", info)
	}
	scope, err = w.Resolve(7, 1, ".")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := scope.Scan(context.Background()); err != nil || len(got.Files) != 1 {
		t.Fatalf("relative changed: %+v %v", got, err)
	}
}

func TestLocalJSONStrictValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	local := LocalRoot{HostRoot: `F:\Local`, ContainerRoot: t.TempDir()}
	for _, config := range []SourceConfig{{LocalRoot: &local}, {Bindings: []Binding{}, LocalRoot: &local}, {Bindings: []Binding{}}} {
		encoded, _ := json.Marshal(config)
		if err := os.WriteFile(file, encoded, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSourceConfig(file); err != nil {
			t.Fatalf("valid config rejected: %v", err)
		}
	}
	for _, body := range []string{`{}`, `{"local_root":null}`, `{"local_root":{"owner_id":1}}`, `{"bindings":[],"unknown":true}`, `{"bindings":[]} {}`} {
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSourceConfig(file); err == nil {
			t.Fatalf("invalid config accepted: %s", body)
		}
	}
	for _, host := range []string{`F:\`, "/", `F:\Local\..\Other`, `\\host\share`, `F:\Local\file:stream`} {
		bad := local
		bad.HostRoot = host
		if _, err := NewWithConfig(t.TempDir(), nil, &bad); err == nil {
			t.Fatalf("invalid local root accepted: %q", host)
		}
	}
}
