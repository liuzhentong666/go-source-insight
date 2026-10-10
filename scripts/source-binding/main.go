// source-binding prepares local read-only mounts; it never deploys or indexes.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type binding struct {
	OwnerID       uint64   `json:"owner_id"`
	ProjectID     uint64   `json:"project_id"`
	HostRoot      string   `json:"host_root"`
	ContainerRoot string   `json:"container_root"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
}
type registry struct {
	Bindings  []binding  `json:"bindings"`
	LocalRoot *localRoot `json:"local_root,omitempty"`
}
type localRoot struct {
	HostRoot      string   `json:"host_root"`
	ContainerRoot string   `json:"container_root"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
}

const localContainerRoot = "/bound-sources/local"

type excludes []string

func (e *excludes) String() string     { return strings.Join(*e, ",") }
func (e *excludes) Set(v string) error { *e = append(*e, v); return nil }

func main() {
	var owner, project uint64
	var host, state, local, activeAPI string
	var exclude excludes
	var replace bool
	var printCompose bool
	flag.Uint64Var(&owner, "owner-id", 0, "Project owner ID")
	flag.Uint64Var(&project, "project-id", 0, "Project ID")
	flag.StringVar(&host, "host-root", "", "Existing absolute directory on the API host")
	flag.StringVar(&state, "state-dir", ".runtime/source-bindings", "Local ignored configuration directory")
	flag.StringVar(&local, "local-root", "", "Private local source directory; no owner/project IDs required")
	flag.StringVar(&activeAPI, "active-api-id", "", "Existing API container used to preserve its active configuration")
	flag.BoolVar(&printCompose, "print-compose-file", false, "Print only the prepared Compose filename")
	flag.Var(&exclude, "exclude", "Root-relative excluded directory (repeatable)")
	flag.BoolVar(&replace, "replace", false, "Explicitly change this project's previously configured root")
	flag.Parse()
	if flag.NArg() != 0 {
		fail(errors.New("Unexpected arguments."))
	}
	if local != "" {
		if owner != 0 || project != 0 || host != "" || replace {
			fail(errors.New("Local source configuration does not accept project IDs or project replacement options."))
		}
		var err error
		var active string
		state, active, err = deploymentState(state, activeAPI)
		if err != nil {
			fail(err)
		}
		if err = configureRoot(0, 0, local, state, exclude, true, true, printCompose, active); err != nil {
			fail(err)
		}
		if printCompose {
			fmt.Println(filepath.Join(state, "source-bindings.compose.json"))
		}
		return
	}
	if activeAPI != "" || printCompose {
		fail(errors.New("Deployment options require --local-root."))
	}
	if err := configure(owner, project, host, state, exclude, replace); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func containerRoot(owner, project uint64) string {
	return "/bound-sources/owners/" + strconv.FormatUint(owner, 10) + "/projects/" + strconv.FormatUint(project, 10)
}
func safeText(value string) bool { return !strings.ContainsAny(value, "\x00\r\n") }
func validHostSyntax(value string) bool {
	if !safeText(value) || !filepath.IsAbs(value) || filepath.Clean(value) != value || filepath.Dir(value) == value || strings.HasPrefix(value, `\\`) || strings.HasPrefix(value, "//") {
		return false
	}
	parts := value
	if runtime.GOOS == "windows" {
		if len(value) < 3 || value[1] != ':' {
			return false
		}
		parts = strings.ReplaceAll(value[3:], `\`, "/")
	} else {
		if strings.ContainsAny(value, `\:`) {
			return false
		}
		parts = strings.TrimPrefix(value, "/")
	}
	_, err := cleanExcludes([]string{parts})
	return err == nil
}
func checkAncestors(directory string) error {
	for current := directory; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return errors.New("The source/configuration path or an ancestor does not exist or cannot be inspected.")
		}
		if isUnsafeLink(info) {
			return errors.New("Symbolic links, junctions and reparse points are not accepted in source/configuration roots or their ancestors.")
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	return nil
}
func canonicalDirectory(input string) (string, error) {
	if !safeText(input) || !filepath.IsAbs(input) || strings.HasPrefix(input, `\\?\`) || strings.HasPrefix(input, `\\.\`) {
		return "", errors.New("Host root must be a regular absolute directory on this machine.")
	}
	clean := filepath.Clean(input)
	if filepath.Dir(clean) == clean {
		return "", errors.New("A filesystem/drive root cannot be authorized.")
	}
	if err := checkAncestors(clean); err != nil {
		return "", err
	}
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return "", errors.New("Bind a directory, not a file. A single .go file can be selected later within an authorized directory.")
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", errors.New("Host root cannot be canonicalized.")
	}
	f, err := os.Open(resolved)
	if err != nil {
		return "", errors.New("Host root is not readable.")
	}
	_, readErr := f.ReadDir(1)
	closeErr := f.Close()
	if (readErr != nil && readErr != io.EOF) || closeErr != nil {
		return "", errors.New("Host root is not readable.")
	}
	resolved = filepath.Clean(resolved)
	if !validHostSyntax(resolved) {
		return "", errors.New("Host directory syntax cannot be represented by the API binding contract.")
	}
	return resolved, nil
}
func cleanExcludes(values []string) ([]string, error) {
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.ReplaceAll(v, `\`, "/")
		if !safeText(v) || v == "" || strings.HasPrefix(v, "/") || strings.Contains(v, ":") {
			return nil, errors.New("Excluded paths must be project-root-relative directories.")
		}
		for _, part := range strings.Split(v, "/") {
			if part == ".." {
				return nil, errors.New("Excluded paths cannot contain parent traversal.")
			}
		}
		v = path.Clean(v)
		if v == "." {
			return nil, errors.New("Cannot exclude the entire source root.")
		}
		for _, component := range strings.Split(v, "/") {
			if strings.TrimRight(component, ". ") != component {
				return nil, errors.New("Excluded paths cannot contain trailing-dot/space aliases.")
			}
			base := strings.ToUpper(strings.Split(component, ".")[0])
			if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
				return nil, errors.New("Excluded paths cannot contain reserved device aliases.")
			}
		}
		key := v
		if runtime.GOOS == "windows" {
			key = strings.ToLower(v)
		}
		if _, exists := seen[key]; !exists {
			seen[key] = true
		}
	}
	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	sort.Strings(result)
	return result, nil
}
func repoExcludes(host string) []string {
	// Match this repository layout, not every business directory named sources.
	for _, name := range []string{"go.mod", "docker-compose.yml", "scripts/deploy.ps1"} {
		info, err := os.Lstat(filepath.Join(host, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
	}
	return []string{"sources", "volumes"}
}
func decodeRegistry(data []byte) (registry, error) {
	var r registry
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil {
		return r, errors.New("Existing binding JSON is invalid; no configuration was changed.")
	}
	if r.Bindings == nil && r.LocalRoot == nil {
		return r, errors.New("Existing binding JSON must contain bindings or a local root; no configuration was changed.")
	}
	if r.Bindings == nil {
		r.Bindings = []binding{}
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return r, errors.New("Existing binding JSON contains extra values; no configuration was changed.")
	}
	seen := map[string]bool{}
	if r.LocalRoot != nil {
		b := r.LocalRoot
		clean, err := cleanExcludes(b.ExcludePaths)
		if !validHostSyntax(b.HostRoot) || b.ContainerRoot != localContainerRoot || err != nil || (len(b.ExcludePaths) != 0 && !reflect.DeepEqual(clean, b.ExcludePaths)) {
			return r, errors.New("Existing local root contains unmanaged roots or exclusions; no configuration was changed.")
		}
		b.ExcludePaths = clean
	}
	for _, b := range r.Bindings {
		key := fmt.Sprintf("%d/%d", b.OwnerID, b.ProjectID)
		if b.OwnerID == 0 || b.ProjectID == 0 || seen[key] || b.ContainerRoot != containerRoot(b.OwnerID, b.ProjectID) || !validHostSyntax(b.HostRoot) {
			return r, errors.New("Existing binding JSON contains invalid/duplicate IDs or unmanaged roots; no configuration was changed.")
		}
		if _, err := cleanExcludes(b.ExcludePaths); err != nil {
			return r, err
		}
		if normalized, _ := cleanExcludes(b.ExcludePaths); !reflect.DeepEqual(normalized, b.ExcludePaths) && len(b.ExcludePaths) != 0 {
			return r, errors.New("Existing binding JSON contains noncanonical/duplicate exclusions; no configuration was changed.")
		}
		seen[key] = true
	}
	return r, nil
}
func composeJSON(r registry, configFile string) ([]byte, error) {
	escape := func(v string) string { return strings.ReplaceAll(v, "$", "$$") }
	mount := func(source, target string) map[string]any {
		return map[string]any{"type": "bind", "source": escape(source), "target": target, "read_only": true, "bind": map[string]any{"create_host_path": false}}
	}
	volumes := []any{mount(configFile, "/app/source-bindings.json")}
	if r.LocalRoot != nil {
		volumes = append(volumes, mount(r.LocalRoot.HostRoot, r.LocalRoot.ContainerRoot))
	}
	for _, b := range r.Bindings {
		volumes = append(volumes, mount(b.HostRoot, b.ContainerRoot))
	}
	return json.MarshalIndent(map[string]any{"services": map[string]any{"api": map[string]any{"environment": map[string]string{"SOURCE_BINDINGS_FILE": "/app/source-bindings.json"}, "volumes": volumes}}}, "", "  ")
}
func writeTemp(directory string, data []byte) (string, error) {
	f, err := os.CreateTemp(directory, ".source-binding-*")
	if err != nil {
		return "", errors.New("Cannot stage binding configuration.")
	}
	name := f.Name()
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", errors.New("Cannot stage binding configuration.")
	}
	return name, nil
}
func loadCommitted(state, composeFile string) (registry, []byte, error) {
	data, err := os.ReadFile(composeFile)
	if errors.Is(err, os.ErrNotExist) {
		// Compatibility with a manually prepared registry; never overwrite it.
		legacy, e := os.ReadFile(filepath.Join(state, "source-bindings.json"))
		if errors.Is(e, os.ErrNotExist) {
			return registry{Bindings: []binding{}}, nil, nil
		}
		if e != nil {
			return registry{}, nil, errors.New("Cannot read existing binding JSON.")
		}
		r, e := decodeRegistry(legacy)
		return r, nil, e
	}
	if err != nil {
		return registry{}, nil, errors.New("Cannot read existing binding Compose configuration.")
	}
	var doc struct {
		Services map[string]struct {
			Volumes []struct {
				Source string `json:"source"`
			} `json:"volumes"`
		} `json:"services"`
	}
	if json.Unmarshal(data, &doc) != nil || len(doc.Services["api"].Volumes) == 0 {
		return registry{}, nil, errors.New("Existing binding Compose configuration is invalid; no configuration was changed.")
	}
	file := strings.ReplaceAll(doc.Services["api"].Volumes[0].Source, "$$", "$")
	if filepath.Dir(file) != state || !regexp.MustCompile(`^source-bindings-[a-f0-9]{64}\.json$`).MatchString(filepath.Base(file)) {
		return registry{}, nil, errors.New("Existing binding Compose configuration references an unmanaged JSON file.")
	}
	info, err := os.Lstat(file)
	if err != nil || isUnsafeLink(info) || !info.Mode().IsRegular() {
		return registry{}, nil, errors.New("Committed binding JSON is missing or is not a regular file.")
	}
	config, err := os.ReadFile(file)
	if err != nil {
		return registry{}, nil, errors.New("Cannot read committed binding JSON.")
	}
	digest := sha256.Sum256(config)
	if filepath.Base(file) != fmt.Sprintf("source-bindings-%x.json", digest) {
		return registry{}, nil, errors.New("Committed binding JSON has changed unexpectedly; no configuration was changed.")
	}
	r, err := decodeRegistry(config)
	if err != nil {
		return registry{}, nil, err
	}
	expected, err := composeJSON(r, file)
	if err != nil {
		return registry{}, nil, errors.New("Cannot validate existing binding Compose configuration.")
	}
	var actualObject, expectedObject any
	if json.Unmarshal(data, &actualObject) != nil || json.Unmarshal(expected, &expectedObject) != nil || !reflect.DeepEqual(actualObject, expectedObject) {
		return registry{}, nil, errors.New("Existing binding Compose configuration contains unmanaged changes; no configuration was changed.")
	}
	return r, data, nil
}

var commitFile = os.Rename // Tests simulate replacement failure without touching services.

func configure(owner, project uint64, host, state string, exclude []string, replace bool) error {
	return configureRoot(owner, project, host, state, exclude, replace, false, false, "")
}
func configureRoot(owner, project uint64, host, state string, exclude []string, replace, local, quiet bool, active string) error {
	if !local && (owner == 0 || project == 0) {
		return errors.New("Provide nonzero owner/project IDs returned by the authenticated API.")
	}
	root, err := canonicalDirectory(host)
	if err != nil {
		return err
	}
	if !safeText(state) {
		return errors.New("Invalid local configuration directory.")
	}
	state, err = filepath.Abs(state)
	if err != nil {
		return errors.New("Invalid local configuration directory.")
	}
	_, stateErr := os.Lstat(state)
	newState := errors.Is(stateErr, os.ErrNotExist)
	for existing := state; ; existing = filepath.Dir(existing) {
		if _, e := os.Lstat(existing); e == nil {
			if err = checkAncestors(existing); err != nil {
				return err
			}
			break
		} else if !errors.Is(e, os.ErrNotExist) {
			return errors.New("Cannot inspect configuration-directory ancestors.")
		}
		if filepath.Dir(existing) == existing {
			return errors.New("Cannot locate configuration-directory parent.")
		}
	}
	if err = os.MkdirAll(state, 0700); err != nil {
		return errors.New("Cannot prepare local configuration directory.")
	}
	if err = checkAncestors(state); err != nil {
		return err
	}
	if err = restrictDirectory(state, newState); err != nil {
		return err
	}
	lockPath := filepath.Join(state, ".source-binding.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("Another binding update is active, or a stale lock requires manual inspection.")
	}
	lock.Close()
	defer os.Remove(lockPath)
	composeFile := filepath.Join(state, "source-bindings.compose.json")
	for _, file := range []string{filepath.Join(state, "source-bindings.json"), composeFile} {
		if info, e := os.Lstat(file); e == nil && (isUnsafeLink(info) || !info.Mode().IsRegular()) {
			return errors.New("Existing binding configuration must be a regular file, not a link.")
		}
	}
	r, oldCompose, err := loadCommitted(state, composeFile)
	if err != nil {
		return err
	}
	if active != "" {
		if err := validateActiveState(r, oldCompose, active, root, exclude); err != nil {
			return err
		}
	}
	position := -1
	if local && r.LocalRoot != nil && r.LocalRoot.HostRoot == root {
		exclude = append(exclude, r.LocalRoot.ExcludePaths...)
	}
	for i, b := range r.Bindings {
		if !local && b.OwnerID == owner && b.ProjectID == project {
			position = i
			if b.HostRoot != root && !replace {
				return errors.New("This project already has another root. Use --replace deliberately; existing indexes are not changed automatically.")
			}
			exclude = append(exclude, b.ExcludePaths...)
		}
	}
	exclude = append(exclude, repoExcludes(root)...)
	clean, err := cleanExcludes(exclude)
	if err != nil {
		return err
	}
	b := binding{owner, project, root, containerRoot(owner, project), clean}
	if local {
		r.LocalRoot = &localRoot{root, localContainerRoot, clean}
		if r.Bindings == nil {
			r.Bindings = []binding{}
		}
	} else if position < 0 {
		r.Bindings = append(r.Bindings, b)
	} else {
		r.Bindings[position] = b
	}
	sort.Slice(r.Bindings, func(i, j int) bool {
		if r.Bindings[i].OwnerID == r.Bindings[j].OwnerID {
			return r.Bindings[i].ProjectID < r.Bindings[j].ProjectID
		}
		return r.Bindings[i].OwnerID < r.Bindings[j].OwnerID
	})
	configData, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errors.New("Cannot encode binding configuration.")
	}
	configData = append(configData, '\n')
	if len(configData) > 1<<20 {
		return errors.New("Binding JSON exceeds the API configuration limit; no configuration was changed.")
	}
	digest := sha256.Sum256(configData)
	configFile := filepath.Join(state, fmt.Sprintf("source-bindings-%x.json", digest))
	composeData, err := composeJSON(r, configFile)
	if err != nil {
		return errors.New("Cannot encode binding mounts.")
	}
	if bytes.Equal(bytes.TrimSpace(oldCompose), composeData) {
		if !quiet {
			fmt.Println("Source binding is unchanged. No services or indexes were modified.")
		}
		return nil
	}
	if info, e := os.Lstat(configFile); e == nil {
		if isUnsafeLink(info) || !info.Mode().IsRegular() {
			return errors.New("An immutable binding generation is not a regular file; no configuration was changed.")
		}
		previous, readErr := os.ReadFile(configFile)
		if readErr != nil || !bytes.Equal(previous, configData) {
			return errors.New("An immutable binding generation is invalid; no configuration was changed.")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return errors.New("Cannot inspect immutable binding generation.")
	} else {
		configTemp, e := writeTemp(state, bytes.TrimSpace(configData))
		if e != nil {
			return e
		}
		defer os.Remove(configTemp)
		// The directory is owner-only. The mounted file must also be readable by
		// the API's distinct non-root UID on native Linux Docker hosts.
		if e = os.Chmod(configTemp, 0644); e != nil {
			return errors.New("Cannot prepare read-only API access to the protected binding generation.")
		}
		if e = os.Rename(configTemp, configFile); e != nil {
			return errors.New("Cannot save immutable binding generation.")
		}
	}
	composeTemp, err := writeTemp(state, composeData)
	if err != nil {
		return err
	}
	defer os.Remove(composeTemp)
	if err = commitFile(composeTemp, composeFile); err != nil {
		return errors.New("Cannot replace local Compose binding mounts; the previous committed mapping remains unchanged.")
	}
	if !quiet {
		fmt.Println("Prepared local read-only source bindings. No services, credentials or indexes were modified.")
		fmt.Println("Apply the generated Compose override after the API/Web files during an arranged API update. See docs/SOURCE_BINDINGS.md.")
		fmt.Println("Changing source roots or exclusions never rebuilds an existing index. Index again only after explicit confirmation.")
	}
	return nil
}

// deploymentState reads only mount metadata, never a container's environment.
// An active immutable generation must match the current entry before an update.
func deploymentState(fallback, containerID string) (string, string, error) {
	state, err := filepath.Abs(fallback)
	if err != nil || !safeText(fallback) {
		return "", "", errors.New("Invalid local configuration directory.")
	}
	if containerID == "" {
		return state, "", nil
	}
	if !regexp.MustCompile(`^[a-f0-9]{12,64}$`).MatchString(containerID) {
		return "", "", errors.New("Cannot identify a single API container in this Compose project.")
	}
	output, err := exec.Command("docker", "inspect", "--format", "{{json .Mounts}}", containerID).Output()
	if err != nil {
		return "", "", errors.New("Cannot inspect existing API mount metadata; existing configuration was not changed.")
	}
	var mounts []struct {
		Source, Destination string
		RW                  bool
	}
	if json.Unmarshal(output, &mounts) != nil {
		return "", "", errors.New("Existing API mount metadata is invalid.")
	}
	active := ""
	for _, mount := range mounts {
		if mount.Destination != "/app/source-bindings.json" {
			continue
		}
		if active != "" || mount.RW {
			return "", "", errors.New("Existing API source configuration is not a single read-only mount.")
		}
		active = mount.Source
	}
	if active == "" {
		return state, "<unconfigured>", nil
	}
	return stateFromActive(state, active)
}
func stateFromActive(fallback, active string) (string, string, error) {
	if active == "" {
		return fallback, "", nil
	}
	// Docker Desktop may report the Linux translation of a Windows drive path.
	if runtime.GOOS == "windows" {
		for _, prefix := range []string{"/run/desktop/mnt/host/", "/host_mnt/"} {
			if strings.HasPrefix(active, prefix) {
				p := strings.TrimPrefix(active, prefix)
				if len(p) > 2 && p[1] == '/' {
					active = strings.ToUpper(p[:1]) + `:\` + filepath.FromSlash(p[2:])
				}
			}
		}
	}
	if !filepath.IsAbs(active) || !safeText(active) {
		return "", "", errors.New("The active API source configuration has an unsupported host path.")
	}
	state := filepath.Dir(active)
	if err := checkAncestors(state); err != nil {
		return "", "", err
	}
	if err := restrictDirectory(state, false); err != nil {
		return "", "", err
	}
	return state, active, nil
}
func validateActiveState(pending registry, data []byte, active, requestedRoot string, exclude []string) error {
	if len(data) == 0 {
		if active == "<unconfigured>" {
			return nil
		}
		return errors.New("The active API source configuration has no managed Compose entry.")
	}
	var doc struct {
		Services map[string]struct{ Volumes []struct{ Source string } }
	}
	if json.Unmarshal(data, &doc) != nil {
		return errors.New("Cannot validate the active source configuration.")
	}
	committed := strings.ReplaceAll(doc.Services["api"].Volumes[0].Source, "$$", "$")
	same := filepath.Clean(committed) == filepath.Clean(active)
	if runtime.GOOS == "windows" {
		same = strings.EqualFold(filepath.Clean(committed), filepath.Clean(active))
	}
	if same {
		return nil
	}
	old := registry{Bindings: []binding{}}
	if active != "<unconfigured>" {
		info, err := os.Lstat(active)
		if err != nil || isUnsafeLink(info) || !info.Mode().IsRegular() {
			return errors.New("The active source generation is missing or unsafe; no configuration was changed.")
		}
		oldData, err := os.ReadFile(active)
		digest := sha256.Sum256(oldData)
		if err != nil || filepath.Base(active) != fmt.Sprintf("source-bindings-%x.json", digest) {
			return errors.New("The active source generation is invalid; no configuration was changed.")
		}
		old, err = decodeRegistry(oldData)
		if err != nil {
			return err
		}
	}
	if pending.LocalRoot != nil && pending.LocalRoot.HostRoot == requestedRoot {
		exclude = append(exclude, pending.LocalRoot.ExcludePaths...)
	}
	exclude = append(exclude, repoExcludes(requestedRoot)...)
	clean, err := cleanExcludes(exclude)
	if err == nil && reflect.DeepEqual(old.Bindings, pending.Bindings) && reflect.DeepEqual(pending.LocalRoot, &localRoot{requestedRoot, localContainerRoot, clean}) {
		return nil // Safe retry of precisely the already prepared directory.
	}
	return errors.New("A different source configuration is waiting to be applied. Retry its original directory or review it before changing the local directory; no pending configuration was overwritten.")
}
