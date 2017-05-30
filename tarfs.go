package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type tarfs struct {
	root dir
}

func (t *tarfs) setName(name string) {
	t.root.setName(name)
}

func (t *tarfs) name() string {
	return t.root.name()
}

func (t *tarfs) files() []file {
	return t.root.files()
}

func (t *tarfs) dirs() []dir {
	return t.root.dirs()
}

func (t *tarfs) addEmptyDir(name string) dir {
	return t.root.addEmptyDir(name)
}

func (t *tarfs) addDir(d dir) dir {
	return t.root.addDir(d)
}

func (t *tarfs) setRecursiveDir(name string, d dir) bool {
	return t.root.setRecursiveDir(name, d)
}

func (t *tarfs) addFile(f file) file {
	return t.root.addFile(f)
}

func (t *tarfs) findDir(name string) dir {
	return t.root.findDir(name)
}

func (t *tarfs) findFile(name string) file {
	return t.root.findFile(name)
}

func newDirFromTar(r io.ReadSeeker) (dir, error) {
	root := new(plainDir)
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// TODO: distinguish files and directories:
		// TypeReg, TypeRegA, TypeDir
		base := path.Dir(h.Name)
		d := recursiveAddDir(root, base)
		// NOTE: this can't be concurrent if same reedseeker will be move
		// back to begining at each readCloser call.
		if h.Typeflag == tar.TypeDir {
			d.addEmptyDir(path.Base(h.Name))
		} else {
			d.addFile(&tarfile{h, r})
		}
	}
	return &tarfs{root}, nil
}

func newStaticTreeFsFromTar(r io.ReadSeeker) (*StaticTreeFs, error) {
	root, err := newDirFromTar(r)
	if err != nil {
		return nil, err
	}
	return &StaticTreeFs{pathfs.NewDefaultFileSystem(), root}, nil
}

func NewTarFs(r io.ReadSeeker) (pathfs.FileSystem, error) {
	fs, err := newStaticTreeFsFromTar(r)
	if err != nil {
		return nil, err
	}
	return pathfs.NewLockingFileSystem(fs), nil
}

type tarfile struct {
	h *tar.Header
	r io.ReadSeeker
}

type dummycloser struct {
	io.Reader
}

func (_ dummycloser) Close() error { return nil }

func (f *tarfile) name() string {
	return path.Base(f.h.Name)
}

func (f *tarfile) size() (uint64, error) {
	return uint64(f.h.Size), nil
}

func (f *tarfile) readCloser() (io.ReadCloser, error) {
	_, err := f.r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek: %v", err)
	}
	tr := tar.NewReader(f.r)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, err
		}
		if h.Name != f.h.Name {
			continue
		}
		var b bytes.Buffer
		n, err := io.Copy(&b, tr)
		if n != h.Size {
			return nil, err
		}
		return dummycloser{&b}, nil
	}
}
