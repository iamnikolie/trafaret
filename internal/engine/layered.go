package engine

import (
	"errors"
	"io"
	"io/fs"
	"sort"
)

// --- templates overlay ----------------------------------------------------

// layeredFS serves top over base: a file present in top shadows the file with
// the same path in base, directories are merged, and files only in top are
// added. When top and base disagree on a path's kind (file vs directory), top wins.
type layeredFS struct{ top, base fs.FS }

func layerFS(top, base fs.FS) fs.FS { return layeredFS{top: top, base: base} }

var (
	_ fs.ReadFileFS = layeredFS{}
	_ fs.ReadDirFS  = layeredFS{}
	_ fs.StatFS     = layeredFS{}
)

func (l layeredFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if fi, err := fs.Stat(l.top, name); err == nil {
		return fi, nil
	}
	return fs.Stat(l.base, name)
}

func (l layeredFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	if fi, err := fs.Stat(l.top, name); err == nil {
		if fi.IsDir() {
			return nil, &fs.PathError{Op: "readfile", Path: name, Err: errors.New("is a directory")}
		}
		return fs.ReadFile(l.top, name)
	}
	return fs.ReadFile(l.base, name)
}

func (l layeredFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	topInfo, topErr := fs.Stat(l.top, name)
	if topErr == nil && !topInfo.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
	}
	baseInfo, baseErr := fs.Stat(l.base, name)
	baseIsDir := baseErr == nil && baseInfo.IsDir()
	if topErr != nil && !baseIsDir {
		if baseErr == nil {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
		}
		return nil, baseErr
	}
	merged := map[string]fs.DirEntry{}
	if baseIsDir {
		entries, err := fs.ReadDir(l.base, name)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			merged[e.Name()] = e
		}
	}
	if topErr == nil {
		entries, err := fs.ReadDir(l.top, name)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			merged[e.Name()] = e
		}
	}
	out := make([]fs.DirEntry, 0, len(merged))
	for _, e := range merged {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func (l layeredFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	fi, err := l.Stat(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if !fi.IsDir() {
		if _, err := fs.Stat(l.top, name); err == nil {
			return l.top.Open(name)
		}
		return l.base.Open(name)
	}
	entries, err := l.ReadDir(name)
	if err != nil {
		return nil, err
	}
	return &mergedDir{info: fi, entries: entries}, nil
}

// mergedDir is the fs.File of a directory in a layeredFS.
type mergedDir struct {
	info    fs.FileInfo
	entries []fs.DirEntry
	off     int
}

func (d *mergedDir) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *mergedDir) Close() error               { return nil }
func (d *mergedDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.Name(), Err: errors.New("is a directory")}
}
func (d *mergedDir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.off:]
	if n <= 0 {
		d.off = len(d.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.off += n
	return rest[:n], nil
}
