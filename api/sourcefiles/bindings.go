package sourcefiles

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go-ai-study/api/fault"
	"go-ai-study/internal/source"
)

// Binding is local runtime configuration, never a client input.
type Binding struct {
	OwnerID       uint     `json:"owner_id"`
	ProjectID     uint     `json:"project_id"`
	HostRoot      string   `json:"host_root"`
	ContainerRoot string   `json:"container_root"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
}

// LocalRoot selects the directory shared by absolute-path operations in private local use.
type LocalRoot struct {
	HostRoot      string   `json:"host_root"`
	ContainerRoot string   `json:"container_root"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
}
type SourceConfig struct {
	Bindings  []Binding  `json:"bindings"`
	LocalRoot *LocalRoot `json:"local_root,omitempty"`
}

type SourceInfo struct {
	SourceMode           string   `json:"source_mode"`
	BindingID            string   `json:"binding_id"`
	ProjectID            uint     `json:"project_id"`
	BindingConfigured    bool     `json:"binding_configured"`
	AbsoluteInputAllowed bool     `json:"absolute_input_allowed"`
	HostRoot             string   `json:"host_root,omitempty"`
	Available            bool     `json:"available"`
	UnavailableCode      string   `json:"unavailable_code,omitempty"`
	ExcludedPaths        []string `json:"excluded_paths"`
}

type bindingKey struct{ owner, project uint }

// LoadBindings preserves the original project-binding loader API.
func LoadBindings(filename string) ([]Binding, error) {
	config, err := LoadSourceConfig(filename)
	return config.Bindings, err
}

// LoadSourceConfig uses bounded strict JSON and omits paths from errors.
func LoadSourceConfig(filename string) (SourceConfig, error) {
	if filename == "" {
		return SourceConfig{}, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return SourceConfig{}, fmt.Errorf("SOURCE_BINDINGS_FILE could not be read")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return SourceConfig{}, fmt.Errorf("SOURCE_BINDINGS_FILE must be a regular JSON file under 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, (1<<20)+1))
	decoder.DisallowUnknownFields()
	var document *SourceConfig
	if err := decoder.Decode(&document); err != nil || document == nil || (document.Bindings == nil && document.LocalRoot == nil) {
		return SourceConfig{}, fmt.Errorf("SOURCE_BINDINGS_FILE contains invalid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return SourceConfig{}, fmt.Errorf("SOURCE_BINDINGS_FILE must contain one JSON object")
	}
	if _, err := validateBindings(document.Bindings); err != nil {
		return SourceConfig{}, err
	}
	if _, err := validateLocalRoot(document.LocalRoot); err != nil {
		return SourceConfig{}, err
	}
	return *document, nil
}

func validateLocalRoot(local *LocalRoot) (*Binding, error) {
	if local == nil {
		return nil, nil
	}
	validated, err := validateBindings([]Binding{{OwnerID: 1, ProjectID: 1, HostRoot: local.HostRoot, ContainerRoot: local.ContainerRoot, ExcludePaths: local.ExcludePaths}})
	if err != nil {
		return nil, err
	}
	binding := validated[bindingKey{1, 1}]
	return &binding, nil
}

func validateBindings(bindings []Binding) (map[bindingKey]Binding, error) {
	result := make(map[bindingKey]Binding, len(bindings))
	for _, binding := range bindings {
		key := bindingKey{binding.OwnerID, binding.ProjectID}
		if key.owner == 0 || key.project == 0 {
			return nil, fmt.Errorf("source binding requires nonzero owner_id and project_id")
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate project source binding")
		}
		host, windows, err := absoluteComponents(binding.HostRoot)
		if err != nil || len(host) < 2 {
			return nil, fmt.Errorf("source binding host_root must be an absolute non-volume-root directory")
		}
		// The readable root must belong to the API's native OS, not the display OS.
		if !filepath.IsAbs(binding.ContainerRoot) || filepath.Clean(binding.ContainerRoot) != binding.ContainerRoot || filepath.Dir(binding.ContainerRoot) == binding.ContainerRoot || strings.ContainsRune(binding.ContainerRoot, '\x00') {
			return nil, fmt.Errorf("source binding container_root must be a native absolute non-volume-root directory")
		}
		if _, _, err := absoluteComponents(binding.ContainerRoot); err != nil {
			return nil, fmt.Errorf("invalid source binding container_root")
		}
		if windows {
			binding.HostRoot = strings.Join(host, "\\")
		} else {
			binding.HostRoot = "/" + strings.Join(host[1:], "/")
		}
		seen := map[string]bool{}
		for _, excluded := range binding.ExcludePaths {
			clean, err := NormalizeTarget(excluded)
			if err != nil || clean == "." || clean == "" {
				return nil, fmt.Errorf("exclude_paths must contain project-relative directory paths")
			}
			key := excluded
			if windows || runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			if seen[key] {
				return nil, fmt.Errorf("duplicate source binding exclusion")
			}
			seen[key] = true
		}
		binding.ExcludePaths = append([]string(nil), binding.ExcludePaths...)
		result[key] = binding
	}
	return result, nil
}

// Components retain OS-independent host syntax. No cleaning may hide traversal.
func absoluteComponents(value string) ([]string, bool, error) {
	windows := len(value) >= 2 && value[1] == ':'
	if strings.ContainsRune(value, '\x00') || strings.HasPrefix(value, "\\") || strings.HasPrefix(value, "//") {
		return nil, windows, fault.ErrInvalidTarget
	}
	var prefix, rest string
	if windows {
		if len(value) < 3 || !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) || (value[2] != '/' && value[2] != '\\') {
			return nil, true, fault.ErrInvalidTarget
		}
		prefix, rest = strings.ToUpper(value[:2]), strings.ReplaceAll(value[3:], "\\", "/")
	} else {
		if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\:") {
			return nil, false, fault.ErrInvalidTarget
		}
		prefix, rest = "/", value[1:]
	}
	rest = strings.TrimSuffix(rest, "/")
	components := []string{prefix}
	if rest == "" {
		return components, windows, nil
	}
	for _, component := range strings.Split(rest, "/") {
		if component == "." || component == ".." || component == "" {
			return nil, windows, fault.ErrInvalidTarget
		}
		if _, err := NormalizeTarget(component); err != nil {
			return nil, windows, err
		}
		components = append(components, component)
	}
	return components, windows, nil
}

func (w *Workspace) binding(owner, project uint) (Binding, bool) {
	b, ok := w.bindings[bindingKey{owner, project}]
	return b, ok
}

func (w *Workspace) excluded(owner, project uint, name string, dir bool) bool {
	binding, bound := w.binding(owner, project)
	_, windows, _ := absoluteComponents(binding.HostRoot)
	parts := strings.Split(name, "/")
	if !dir {
		parts = parts[:len(parts)-1]
	}
	for _, part := range parts {
		if windows {
			part = strings.ToLower(part)
		}
		if source.ShouldSkipDir(part) {
			return true
		}
	}
	if bound {
		return w.explicitlyExcluded(owner, project, name)
	}
	return false
}

func (w *Workspace) explicitlyExcluded(owner, project uint, name string) bool {
	binding, bound := w.binding(owner, project)
	_, windows, _ := absoluteComponents(binding.HostRoot)
	if bound {
		for _, excluded := range binding.ExcludePaths {
			candidate := name
			if windows || runtime.GOOS == "windows" {
				candidate, excluded = strings.ToLower(candidate), strings.ToLower(excluded)
			}
			if candidate == excluded || strings.HasPrefix(candidate, excluded+"/") {
				return true
			}
		}
	}
	return false
}

func (w *Workspace) NormalizeTarget(owner, project uint, target string) (string, error) {
	binding, bound := w.binding(owner, project)
	if strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\\:") {
		if !bound {
			return "", fault.ErrInvalidTarget
		}
		root, windows, _ := absoluteComponents(binding.HostRoot)
		input, inputWindows, err := absoluteComponents(target)
		if err != nil || windows != inputWindows || len(input) < len(root) {
			return "", fault.ErrInvalidTarget
		}
		for i := range root {
			equal := root[i] == input[i]
			if windows {
				equal = strings.EqualFold(root[i], input[i])
			}
			if !equal {
				return "", fault.ErrInvalidTarget
			}
		}
		target = strings.Join(input[len(root):], "/")
	}
	clean, err := NormalizeTarget(target)
	if err != nil {
		return "", err
	}
	if w.excluded(owner, project, clean, true) {
		return "", fault.ErrExcludedTarget
	}
	if bound {
		_, windows, _ := absoluteComponents(binding.HostRoot)
		if windows && clean != "." {
			clean, err = w.canonicalWindowsTarget(owner, project, clean)
			if err != nil {
				return "", err
			}
		}
	}
	return clean, nil
}

// Windows host comparisons are case-insensitive even when the API runs in a
// Linux container. Find the actual directory-entry spelling without bypassing
// handle confinement. Missing roots/targets remain durable source failures.
func (w *Workspace) canonicalWindowsTarget(owner, project uint, target string) (string, error) {
	root, err := w.open(owner, project)
	if err != nil {
		if err == fault.ErrUnsafePath {
			return "", err
		}
		return target, nil
	}
	defer root.Close()
	parent := root
	defer func() {
		if parent != root {
			parent.Close()
		}
	}()
	parts := strings.Split(target, "/")
	for i, part := range parts {
		directory, err := parent.Open(".")
		if err != nil {
			return target, nil
		}
		entries, err := directory.ReadDir(-1)
		directory.Close()
		if err != nil {
			return target, nil
		}
		match, err := caseInsensitiveEntry(entries, part)
		if err != nil {
			return "", err
		}
		if match == nil {
			return target, nil
		}
		parts[i] = match.Name()
		if i < len(parts)-1 {
			next, err := openDirectory(parent, match.Name())
			if err != nil {
				return "", sourceError(err)
			}
			if parent != root {
				parent.Close()
			}
			parent = next
		}
	}
	return strings.Join(parts, "/"), nil
}

func caseInsensitiveEntry(entries []fs.DirEntry, name string) (fs.DirEntry, error) {
	var match fs.DirEntry
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			if match != nil {
				return nil, fault.ErrUnsafePath
			}
			match = entry
		}
	}
	return match, nil
}

func (w *Workspace) Info(owner, project uint) SourceInfo {
	info := SourceInfo{SourceMode: "legacy", ProjectID: project, ExcludedPaths: []string{}}
	if w.local != nil {
		scope := w.localWorkspace(owner, project)
		info = scope.Info(owner, project)
		info.SourceMode = "local"
		encoded, _ := json.Marshal(LocalRoot{HostRoot: w.local.HostRoot, ContainerRoot: w.local.ContainerRoot, ExcludePaths: w.local.ExcludePaths})
		info.BindingID = fmt.Sprintf("%x", sha256.Sum256(encoded))
		return info
	}
	if binding, ok := w.binding(owner, project); ok {
		info.SourceMode = "bound"
		info.BindingConfigured, info.AbsoluteInputAllowed, info.HostRoot = true, true, binding.HostRoot
		info.ExcludedPaths = append(info.ExcludedPaths, binding.ExcludePaths...)
		encoded, _ := json.Marshal(binding)
		info.BindingID = fmt.Sprintf("%x", sha256.Sum256(encoded))
	}
	root, err := w.open(owner, project)
	if err == nil {
		file, openErr := root.Open(".")
		if openErr == nil {
			_, openErr = file.ReadDir(1)
			file.Close()
			if openErr == io.EOF {
				openErr = nil
			}
		}
		root.Close()
		err = openErr
	}
	info.Available = err == nil
	if err != nil {
		info.UnavailableCode, _ = fault.Public(sourceError(err))
	}
	return info
}
