package main

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/ulikunitz/xz"
)

type comp struct {
	isSupported func(string) bool
	wrap        func(io.Reader) (io.Reader, error)
}

// ZipFs is a fuse filesystem that mounts zip archives
type ZipFs struct {
	pathfs.FileSystem
	z *zip.Reader
	// caching this way is fast enough for now. If there will be need to make
	// it faster, this could be chanegd to map from prefix to set of files.
	files map[string]*zip.File
	dirs  map[string]struct{}
	comps []comp
}

// NewZipFs returns new filesystem reading zip archive from r of size.
func NewZipFs(r io.ReaderAt, size int64) (pathfs.FileSystem, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	dirs := map[string]struct{}{}
	for _, f := range zipr.File {
		files[f.Name] = f
		p := path.Dir(f.Name)
		for p != "." {
			dirs[p] = struct{}{}
			p = path.Dir(p)
		}
	}
	dirs[""] = struct{}{}
	comps := []comp{
		{isGzip, func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }},
		{isXz, func(r io.Reader) (io.Reader, error) { return xz.NewReader(r) }},
		{isBzip, func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }},
		{isUncompressed, func(r io.Reader) (io.Reader, error) { return r, nil }},
	}
	zfs := &ZipFs{pathfs.NewDefaultFileSystem(), zipr, files, dirs, comps}
	return pathfs.NewLockingFileSystem(zfs), nil
}

func (z *ZipFs) isDir(path string) bool {
	_, ok := z.dirs[path]
	return ok
}

func (z *ZipFs) fileSize(path string) (uint64, bool) {
	f, ok := z.files[path]
	if !ok {
		return 0, false
	}
	if isGzip(path) || isXz(path) || isBzip(path) {
		r, err := f.Open()
		if err != nil {
			return 0, false
		}
		l, err := z.read(path, r)
		if err != nil {
			return 0, false
		}
		return uint64(len(l)), true
	}
	return f.UncompressedSize64, true
}

func isProperPrefix(s, prefix string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	return !(len(prefix) > 0 && len(s) > len(prefix) && s[len(prefix)] != '/')
}

// OpenDir returns list of files and directories directly under path.
func (z *ZipFs) OpenDir(path string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	debugf("OpenDir: '%s'", path)
	if !z.isDir(path) {
		return nil, fuse.ENOENT
	}
	files := make([]fuse.DirEntry, 0)
	seen := map[string]struct{}{}
	for _, e := range z.z.File {
		if !isProperPrefix(e.Name, path) {
			continue
		}
		components := strings.Split(removePrefixPath(e.Name, path), "/")
		// TODO: should I check len here?
		first := components[0]
		if _, ok := seen[first]; ok {
			continue
		}
		mode := mode(len(components) == 1)
		seen[first] = struct{}{}
		files = append(files, fuse.DirEntry{Name: first, Mode: mode})
	}
	return files, 0
}

// GetAttr returns attributes of path.
func (z *ZipFs) GetAttr(path string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	size, isFile := z.fileSize(path)
	if !isFile && !z.isDir(path) {
		return nil, fuse.ENOENT
	}
	attr := &fuse.Attr{Mode: mode(isFile), Size: size}
	debugf("GetAttr: '%s' -> file:%v dir:%v (%v)", path, attr.IsRegular(), attr.IsDir(), attr)
	return attr, fuse.OK
}

// Open return File representing contents stored under path.
func (z *ZipFs) Open(path string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f, ok := z.files[path]
	if !ok {
		return nil, fuse.ENOENT
	}
	r, err := f.Open()
	if err != nil {
		debugf("failed to open '%v': %v", path, err)
		return nil, fuse.EIO // TODO: EIO?
	}
	defer r.Close()
	b, err := z.read(path, r)
	if err != nil {
		debugf("failed to open: %v", err)
		return nil, fuse.EIO
	}
	return nodefs.NewDataFile(b), fuse.OK
}

func (z *ZipFs) read(path string, r io.Reader) ([]byte, error) {
	for _, c := range z.comps {
		if c.isSupported(path) {
			r, err := c.wrap(r)
			if err != nil {
				return nil, err
			}
			return ioutil.ReadAll(r)
		}
	}
	return nil, fmt.Errorf("unsupported format of '%v'", path)
}

func removePrefixPath(s, prefix string) string {
	suffix := strings.TrimPrefix(s, prefix)
	if suffix != "" && suffix[0] == '/' {
		suffix = suffix[1:]
	}
	return suffix
}

func mode(isFile bool) uint32 {
	if isFile {
		return uint32(0755) | fuse.S_IFREG
	}
	return uint32(0755) | fuse.S_IFDIR
}
