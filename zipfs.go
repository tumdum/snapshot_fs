package main

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"io/ioutil"
	"path"
	"strings"
	"syscall"

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

func NewZipFs(r io.ReaderAt, size int64) (*ZipFs, error) {
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
	return &ZipFs{pathfs.NewDefaultFileSystem(), zipr, files, dirs, comps}, nil
}

func (z *ZipFs) isFile(name string) bool {
	_, ok := z.files[name]
	return ok
}

func (z *ZipFs) fileSize(name string) (uint64, bool) {
	f, ok := z.files[name]
	if !ok {
		return 0, false
	}
	if isGzip(name) || isXz(name) || isBzip(name) {
		r, err := f.Open()
		if err != nil {
			return 0, false
		}
		l, err := z.read(name, r)
		if err != nil {
			return 0, false
		}
		return uint64(len(l)), true
	}
	return f.UncompressedSize64, true
}

func (z *ZipFs) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	debugf("OpenDir: %s", name)
	files := make([]fuse.DirEntry, 0)
	seen := map[string]struct{}{}
	for _, entry := range z.z.File {
		if !strings.HasPrefix(entry.Name, name) {
			continue
		}
		suffix := strings.TrimPrefix(entry.Name, name)
		// There are only files in zip file. So OpenDir called on
		// file is an error.
		if suffix == "" {
			return nil, fuse.Status(syscall.ENOSYS)
		}

		components := strings.Split(suffix, "/")
		if components[0] == "" {
			components = components[1:]
		}
		// TODO: should I check len here?
		first := components[0]
		if _, ok := seen[first]; ok {
			continue
		}
		seen[first] = struct{}{}
		debugf("name: %v", first)
		files = append(files, fuse.DirEntry{Name: first, Mode: z.mode(first)})
	}
	if len(files) == 0 && len(z.files) > 0 {
		// Zip files contain only files?
		return nil, fuse.ENOENT
	}
	return files, 0
}

func (z *ZipFs) mode(name string) uint32 {
	mode := uint32(0755)
	if z.isFile(name) {
		mode |= fuse.S_IFREG
	} else {
		mode |= fuse.S_IFDIR
	}
	return mode
}

func (z *ZipFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	// TODO: this returns status ok for not exstisting paths
	size, isFile := z.fileSize(name)
	if !isFile {
		_, isDir := z.dirs[name]
		if !isDir {
			return nil, fuse.ENOENT
		}
	}
	attr := &fuse.Attr{Mode: z.mode(name), Size: size}
	debugf("GetAttr: %s -> file:%v dir:%v", name, attr.IsRegular(), attr.IsDir())
	return attr, fuse.OK
}

func (z *ZipFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f, ok := z.files[name]
	if !ok {
		return nil, fuse.ENOENT
	}
	r, err := f.Open()
	if err != nil {
		debugf("failed to open '%v': %v", name, err)
		return nil, fuse.EIO // TODO: EIO?
	}
	defer r.Close()
	b, err := z.read(name, r)
	if err != nil {
		debugf("failed to open: %v", err)
		return nil, fuse.EIO
	}
	return nodefs.NewDataFile(b), fuse.OK
}

func (z *ZipFs) read(name string, r io.Reader) ([]byte, error) {
	var reader io.Reader

	for _, c := range z.comps {
		if c.isSupported(name) {
			tmp, err := c.wrap(r)
			if err != nil {
				return nil, err
			}
			reader = tmp
			break
		}
	}
	if reader == nil {
		// This should really never happen, unless I screw up and remove
		// last element from comps which supports *all* files.
		panic("read did not find matching reader for '" + name)
	}

	return ioutil.ReadAll(reader)
}
