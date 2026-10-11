package localfiles

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"go-ai-study/internal/source"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type request struct {
	Target string `json:"target"`
	Name   string `json:"name,omitempty"`
}
type metadata struct {
	Name string      `json:"name"`
	Size int64       `json:"size"`
	Dir  bool        `json:"dir"`
	Mode fs.FileMode `json:"mode"`
}
type wire struct {
	Target  string     `json:"target,omitempty"`
	Info    metadata   `json:"info"`
	Entries []metadata `json:"entries,omitempty"`
	Data    []byte     `json:"data,omitempty"`
	Code    string     `json:"code,omitempty"`
}

func meta(info fs.FileInfo) metadata {
	return metadata{info.Name(), info.Size(), info.IsDir(), info.Mode()}
}
func (m metadata) IsDir() bool                { return m.Dir }
func (m metadata) Type() fs.FileMode          { return m.Mode.Type() }
func (m metadata) Info() (fs.FileInfo, error) { return fileInfo{m}, nil }

type fileInfo struct{ metadata }

func (i fileInfo) Name() string       { return i.metadata.Name }
func (i fileInfo) Size() int64        { return i.metadata.Size }
func (i fileInfo) Mode() fs.FileMode  { return i.metadata.Mode }
func (i fileInfo) ModTime() time.Time { return time.Time{} }
func (i fileInfo) Sys() any           { return nil }

// metadata needs a Name method for DirEntry, so directory entries use fileInfo.
func (i fileInfo) Type() fs.FileMode          { return i.metadata.Mode.Type() }
func (i fileInfo) Info() (fs.FileInfo, error) { return i, nil }

func (p Provider) call(ctx context.Context, endpoint string, input request, result *wire) error {
	parsed, err := url.Parse(p.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Host == "" {
		return errors.New("invalid source reader URL")
	}
	body, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL+endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("source reader unavailable")
	}
	if endpoint == "/health" {
		req.Method = http.MethodGet
		req.Body = nil
		req.ContentLength = 0
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("source reader unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		if response.StatusCode == 400 {
			return ErrTarget
		}
		if response.StatusCode == 403 {
			var failure wire
			if json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&failure) == nil && failure.Code == "unsafe" {
				return ErrUnsafe
			}
			return fs.ErrPermission
		}
		if response.StatusCode == 404 {
			return fs.ErrNotExist
		}
		return errors.New("source reader unavailable")
	}
	if endpoint == "/health" {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 24<<20)).Decode(result); err != nil {
		return errors.New("invalid source reader response")
	}
	return nil
}
func (p Provider) Healthy(ctx context.Context) bool {
	if p.URL == "" {
		return true
	}
	return p.call(ctx, "/health", request{}, &wire{}) == nil
}

type remoteFS struct {
	ctx      context.Context
	provider Provider
	target   string
}

func (r *remoteFS) ShouldSkipSourceDir(name string) bool {
	if len(r.target) > 1 && r.target[1] == ':' || strings.HasPrefix(r.target, `\\`) {
		name = strings.ToLower(name)
	}
	return source.ShouldSkipDir(name)
}

func (r *remoteFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
		return nil, fs.ErrPermission
	}
	var value wire
	if err := r.provider.call(r.ctx, "/open", request{r.target, name}, &value); err != nil {
		return nil, err
	}
	return &remoteFile{wire: value, Reader: bytes.NewReader(value.Data)}, nil
}

type remoteFile struct {
	wire wire
	*bytes.Reader
	offset int
}

func (f *remoteFile) Stat() (fs.FileInfo, error) { return fileInfo{f.wire.Info}, nil }
func (f *remoteFile) Close() error               { return nil }
func (f *remoteFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.wire.Info.Dir {
		return nil, fs.ErrInvalid
	}
	remaining := len(f.wire.Entries) - f.offset
	if n > 0 && remaining == 0 {
		return nil, io.EOF
	}
	count := remaining
	if n > 0 && n < count {
		count = n
	}
	entries := make([]fs.DirEntry, count)
	for i := range entries {
		entries[i] = fileInfo{f.wire.Entries[f.offset+i]}
	}
	f.offset += count
	return entries, nil
}

// Handler exposes no writes, commands, CORS or non-Go file contents.
func Handler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(token) < 32 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "authentication required", 401)
			return
		}
		if r.URL.Path == "/health" && r.Method == http.MethodGet {
			w.WriteHeader(200)
			return
		}
		if r.Method != http.MethodPost || (r.URL.Path != "/resolve" && r.URL.Path != "/open") {
			http.NotFound(w, r)
			return
		}
		var input request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		var extra any
		if !errors.Is(decoder.Decode(&extra), io.EOF) {
			http.Error(w, "invalid request", 400)
			return
		}
		files, target, close, err := Open(input.Target)
		if err != nil {
			writeError(w, err)
			return
		}
		defer close()
		result := wire{Target: target}
		if r.URL.Path == "/open" {
			file, e := files.Open(input.Name)
			if e != nil {
				writeError(w, e)
				return
			}
			defer file.Close()
			info, e := file.Stat()
			if e != nil {
				writeError(w, e)
				return
			}
			result.Info = meta(info)
			if info.IsDir() {
				directory, ok := file.(fs.ReadDirFile)
				if !ok {
					writeError(w, fs.ErrInvalid)
					return
				}
				for {
					if r.Context().Err() != nil {
						return
					}
					entries, e := directory.ReadDir(256)
					for _, entry := range entries {
						if entry.IsDir() || source.IsGoFile(entry.Name()) {
							i, e := entry.Info()
							if e != nil {
								writeError(w, e)
								return
							}
							if unsafe(i) {
								continue
							}
							result.Entries = append(result.Entries, meta(i))
							if len(result.Entries) > 100000 {
								writeError(w, fs.ErrPermission)
								return
							}
						}
					}
					if e == io.EOF {
						break
					}
					if e != nil {
						writeError(w, e)
						return
					}
				}
			} else {
				result.Data, e = io.ReadAll(io.LimitReader(file, MaxFileSize+1))
				if e != nil || len(result.Data) > MaxFileSize {
					writeError(w, fs.ErrPermission)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(result)
	})
}
func writeError(w http.ResponseWriter, err error) {
	status := 404
	code := "unavailable"
	if errors.Is(err, ErrTarget) {
		status = 400
		code = "target"
	}
	if errors.Is(err, ErrUnsafe) || errors.Is(err, fs.ErrPermission) {
		status = 403
		code = "permission"
		if errors.Is(err, ErrUnsafe) {
			code = "unsafe"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(wire{Code: code})
}
