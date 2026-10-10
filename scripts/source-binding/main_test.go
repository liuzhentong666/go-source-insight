package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func fixtureDir(t *testing.T, root, name string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "main.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func committed(t *testing.T, state string) (registry, []byte, string) {
	t.Helper()
	r, data, err := loadCommitted(state, filepath.Join(state, "source-bindings.compose.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct {
			Volumes []struct {
				Source   string `json:"source"`
				Target   string `json:"target"`
				ReadOnly bool   `json:"read_only"`
				Bind     struct {
					CreateHostPath bool `json:"create_host_path"`
				} `json:"bind"`
			} `json:"volumes"`
		} `json:"services"`
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, v := range doc.Services["api"].Volumes {
		if !v.ReadOnly || v.Bind.CreateHostPath {
			t.Fatal("Binding mount is writable or auto-creates host paths")
		}
	}
	file := strings.ReplaceAll(doc.Services["api"].Volumes[0].Source, "$$", "$")
	return r, data, file
}

func TestPreserveBindingsCredentialsAndIdempotency(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	first := fixtureDir(t, root, "source with spaces $($null) & semi;")
	second := fixtureDir(t, root, "second source")
	if err := configure(17, 29, first, state, []string{"generated", "nested/generated"}, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "api.env"), []byte("test-only-unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	r, before, generation := committed(t, state)
	if len(r.Bindings) != 1 || r.Bindings[0].HostRoot != first {
		t.Fatal("Wrong first binding")
	}
	if !bytes.Contains(before, []byte("$$")) {
		t.Fatal("Literal dollar signs were not protected against Compose interpolation")
	}
	oldJSON, _ := os.ReadFile(generation)
	if err := configure(31, 47, second, state, nil, false); err != nil {
		t.Fatal(err)
	}
	r, after, _ := committed(t, state)
	if len(r.Bindings) != 2 || r.Bindings[0].HostRoot != first || r.Bindings[1].HostRoot != second {
		t.Fatal("Existing project binding was not preserved")
	}
	if previous, err := os.ReadFile(generation); err != nil || !bytes.Equal(previous, oldJSON) {
		t.Fatal("Old generation used by running containers was altered")
	}
	if credential, _ := os.ReadFile(filepath.Join(state, "api.env")); string(credential) != "test-only-unchanged" {
		t.Fatal("Credentials changed")
	}
	if err := configure(31, 47, second, state, nil, false); err != nil {
		t.Fatal(err)
	}
	_, repeated, _ := committed(t, state)
	if !bytes.Equal(after, repeated) {
		t.Fatal("Repeated update was not idempotent")
	}
	if err := configure(17, 29, second, state, nil, false); err == nil {
		t.Fatal("Changed root accepted without explicit replace")
	}
	_, protected, _ := committed(t, state)
	if !bytes.Equal(after, protected) {
		t.Fatal("Rejected root replacement altered configuration")
	}
	if err := configure(17, 29, second, state, nil, true); err != nil {
		t.Fatal(err)
	}
	r, _, _ = committed(t, state)
	if len(r.Bindings) != 2 || r.Bindings[1].HostRoot != second || len(r.Bindings[0].ExcludePaths) != 2 {
		t.Fatal("Explicit root replacement lost another project or exclusions")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(state); info.Mode().Perm() != 0700 {
			t.Fatal("Configuration directory is not restricted")
		}
		if info, _ := os.Stat(generation); info.Mode().Perm() != 0644 {
			t.Fatal("Protected mounted generation cannot be read by API UID")
		}
	}
}

func TestInvalidSourceAndExistingConfigurationRemainUnchanged(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := configure(17, 29, source, state, nil, false); err != nil {
		t.Fatal(err)
	}
	_, before, generation := committed(t, state)
	for _, invalid := range []string{"relative", filepath.Join(root, "missing"), filepath.Join(source, "main.go"), filepath.VolumeName(source) + string(filepath.Separator), "bad\npath"} {
		if err := configure(17, 29, invalid, state, nil, true); err == nil {
			t.Fatal("Invalid root accepted")
		}
	}
	for _, invalid := range []string{"../escape", "nested/../escape", ".", "/absolute", "C:/absolute", "bad\npath", "aux", "COM1.log", "NUL", "file.", "file "} {
		if err := configure(17, 29, source, state, []string{invalid}, false); err == nil {
			t.Fatal("Invalid excluded path accepted")
		}
	}
	_, unchanged, _ := committed(t, state)
	if !bytes.Equal(before, unchanged) {
		t.Fatal("Rejected configuration changed commit")
	}
	if err := os.WriteFile(generation, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := configure(31, 47, source, state, nil, false); err == nil {
		t.Fatal("Corrupted existing JSON accepted")
	}
	if now, _ := os.ReadFile(filepath.Join(state, "source-bindings.compose.json")); !bytes.Equal(before, now) {
		t.Fatal("Corrupted JSON caused old commit to be replaced")
	}
	invalidState := filepath.Join(root, "invalid-state")
	os.MkdirAll(invalidState, 0700)
	invalid := []byte(`{"bindings":[],"unknown":true}`)
	os.WriteFile(filepath.Join(invalidState, "source-bindings.json"), invalid, 0600)
	if err := configure(17, 29, source, invalidState, nil, false); err == nil {
		t.Fatal("Unknown legacy configuration field accepted")
	}
	if now, _ := os.ReadFile(filepath.Join(invalidState, "source-bindings.json")); !bytes.Equal(now, invalid) {
		t.Fatal("Invalid legacy configuration overwritten")
	}
}

func TestCommitFailurePreservesActiveGeneration(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := configure(17, 29, source, state, nil, false); err != nil {
		t.Fatal(err)
	}
	_, before, generation := committed(t, state)
	old, _ := os.ReadFile(generation)
	previousCommit := commitFile
	commitFile = func(_, _ string) error { return errors.New("simulated interrupted/denied replacement") }
	defer func() { commitFile = previousCommit }()
	if err := configure(31, 47, source, state, nil, false); err == nil {
		t.Fatal("Commit failure was hidden")
	}
	_, active, activeFile := committed(t, state)
	if !bytes.Equal(active, before) || activeFile != generation {
		t.Fatal("Failed replacement switched active generation")
	}
	if data, _ := os.ReadFile(generation); !bytes.Equal(old, data) {
		t.Fatal("Previous mounted JSON changed")
	}
	commitFile = previousCommit
	if err := configure(31, 47, source, state, nil, false); err != nil {
		t.Fatal("Safe retry after interruption failed:", err)
	}
}

func TestLinkedRootsAndConfigurationRejected(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	linked := filepath.Join(root, "linked")
	if err := os.Symlink(source, linked); err != nil {
		t.Skip("Symlink creation requires host permission")
	}
	if err := configure(17, 29, linked, filepath.Join(root, "state"), nil, false); err == nil {
		t.Fatal("Linked root accepted")
	}
	if err := configure(17, 29, filepath.Join(linked, "nested"), filepath.Join(root, "state"), nil, false); err == nil {
		t.Fatal("Linked root ancestor accepted")
	}
	if err := configure(17, 29, source, filepath.Join(linked, "new-state"), nil, false); err == nil {
		t.Fatal("Linked state ancestor accepted")
	}
	if _, err := os.Stat(filepath.Join(source, "new-state")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Rejected state path created files outside allowed directory")
	}
}

func TestRepositoryExcludesOnlyExactRootLocations(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "business")
	if len(repoExcludes(source)) != 0 {
		t.Fatal("Business directories were globally excluded")
	}
	os.MkdirAll(filepath.Join(source, "scripts"), 0700)
	for _, name := range []string{"go.mod", "docker-compose.yml", "scripts/deploy.ps1"} {
		os.WriteFile(filepath.Join(source, filepath.FromSlash(name)), []byte("fixture"), 0600)
	}
	excluded := repoExcludes(source)
	if len(excluded) != 2 || excluded[0] != "sources" || excluded[1] != "volumes" {
		t.Fatal("Repository runtime exclusions missing")
	}
	if normalized, err := cleanExcludes([]string{"generated", "generated/", `nested\output`}); err != nil || len(normalized) != 2 {
		t.Fatal("Exclusion normalization failed")
	}
}

func TestBindingHostSyntaxMatchesAPI(t *testing.T) {
	root := t.TempDir()
	if !validHostSyntax(root) {
		t.Fatal("Regular canonical directory syntax rejected")
	}
	for _, part := range []string{"AUX", "CON.txt", "tail.", "tail ", "nested:alternate"} {
		if validHostSyntax(filepath.Join(root, part)) {
			t.Fatal("Backend-rejected host syntax accepted")
		}
	}
	if validHostSyntax(`\\server\share\project`) {
		t.Fatal("Unsupported UNC host root accepted")
	}
	if runtime.GOOS != "windows" && validHostSyntax(filepath.Join(root, `back\slash`)) {
		t.Fatal("POSIX backslash alias accepted")
	}
}

func TestLocalRootPreservesProjectsAndSafePendingRetry(t *testing.T) {
	root := t.TempDir()
	first := fixtureDir(t, root, "first $(&;) source")
	second := fixtureDir(t, root, "second")
	state := filepath.Join(root, "nondefault state")
	if err := configure(17, 29, first, state, []string{"generated"}, false); err != nil {
		t.Fatal(err)
	}
	old, _, active := committed(t, state)
	if err := os.WriteFile(filepath.Join(state, "api.env"), []byte("unchanged-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	discovered, path, err := stateFromActive(filepath.Join(root, "default"), active)
	if err != nil || discovered != state || path != active {
		t.Fatalf("Active state was not reused: %v", err)
	}
	if err := configureRoot(0, 0, first, state, []string{"scratch"}, true, true, true, active); err != nil {
		t.Fatal(err)
	}
	pending, before, generation := committed(t, state)
	if pending.LocalRoot == nil || pending.LocalRoot.HostRoot != first || pending.LocalRoot.ContainerRoot != localContainerRoot || !reflect.DeepEqual(pending.Bindings, old.Bindings) {
		t.Fatal("Local root lost existing project configuration")
	}
	// A failed build/up leaves the active mount on the previous immutable generation.
	if err := configureRoot(0, 0, first, state, nil, true, true, true, active); err != nil {
		t.Fatal("Identical pending configuration cannot be retried:", err)
	}
	if err := configureRoot(0, 0, second, state, nil, true, true, true, active); err == nil {
		t.Fatal("Different pending root was overwritten")
	}
	_, after, afterGeneration := committed(t, state)
	if !bytes.Equal(before, after) || generation != afterGeneration {
		t.Fatal("Rejected pending replacement altered the entry")
	}
	if data, _ := os.ReadFile(filepath.Join(state, "api.env")); string(data) != "unchanged-fixture" {
		t.Fatal("Credentials were modified")
	}
	if err := configureRoot(0, 0, second, state, nil, true, true, true, generation); err != nil {
		t.Fatal("Applied root could not be changed:", err)
	}
	r, _, _ := committed(t, state)
	if r.LocalRoot.HostRoot != second || !reflect.DeepEqual(r.Bindings, old.Bindings) {
		t.Fatal("Replacement lost project roots")
	}
}

func TestLocalRootFailuresKeepCommittedMapping(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := configureRoot(0, 0, source, state, nil, true, true, true, ""); err != nil {
		t.Fatal(err)
	}
	_, before, active := committed(t, state)
	for _, target := range []string{"relative", filepath.Join(source, "main.go"), filepath.Join(root, "missing")} {
		if err := configureRoot(0, 0, target, state, nil, true, true, true, active); err == nil {
			t.Fatal("Invalid local root accepted")
		}
	}
	previous := commitFile
	commitFile = func(_, _ string) error { return errors.New("fixture failure") }
	other := fixtureDir(t, root, "other")
	err := configureRoot(0, 0, other, state, nil, true, true, true, active)
	commitFile = previous
	if err == nil {
		t.Fatal("Failed local commit reported success")
	}
	_, after, generation := committed(t, state)
	if !bytes.Equal(before, after) || generation != active {
		t.Fatal("Failed local change altered active generation")
	}
	if err := configureRoot(0, 0, source, state, nil, true, true, true, "<unconfigured>"); err != nil {
		t.Fatal("Initial pending configuration cannot be retried:", err)
	}
	if err := configureRoot(0, 0, other, state, nil, true, true, true, "<unconfigured>"); err == nil {
		t.Fatal("Initial pending configuration was overwritten")
	}
}
