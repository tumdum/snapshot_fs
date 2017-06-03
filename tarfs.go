package main

import (
	"archive/tar"
	"bytes"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type tarfs struct {
	dir
}

func findAllPathsInTar(r io.ReadSeeker) (map[string]struct{}, error) {
	m := map[string]struct{}{}
	tr := tar.NewReader(r)
	defer r.Seek(0, io.SeekStart)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(h.Name, "/")
		m[name] = struct{}{}
	}
	return m, nil
}

func newDirFromTar(r io.ReadSeeker) (dir, error) {
	seen, err := findAllPathsInTar(r)
	if err != nil {
		return nil, err
	}
	m := new(sync.Mutex)
	root := new(plainDir)
	tr := tar.NewReader(r)
	var offset int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		offset, err = r.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		base := path.Dir(h.Name)
		d := recursiveAddDir(root, base)
		if h.Typeflag == tar.TypeDir {
			if h.Name[len(h.Name)-1] != '/' {
				d.addEmptyDir(path.Base(h.Name))
			}
		} else {
			if isArchive(h.Name) {
				b := make([]byte, h.Size)
				if _, err := tr.Read(b); err != nil && err != io.EOF {
					return nil, err
				}
				tarDir, err := newDirFromArchive(bytes.NewReader(b), int64(len(b)), h.Name)
				if err != nil {
					return nil, err
				}
				name := notCollidingArchiveName(h.Name, seen)
				tarDir.setName(path.Base(name))
				d.addDir(tarDir)
			} else {
				ext := path.Ext(h.Name)
				name := notCollidingCompressedName(h.Name, seen)
				d.addFile(newFile(&tarFile{h, name, r, m, offset}, ext))
			}
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

func newTarFs(r io.ReadSeeker) (pathfs.FileSystem, error) {
	fs, err := newStaticTreeFsFromTar(r)
	if err != nil {
		return nil, err
	}
	return pathfs.NewLockingFileSystem(fs), nil
}

type tarFile struct {
	h      *tar.Header
	n      string
	r      io.ReadSeeker
	m      *sync.Mutex
	offset int64
}

func (f *tarFile) name() string {
	return path.Base(f.n)
}

func (f *tarFile) size() (uint64, error) {
	return uint64(f.h.Size), nil
}

func (f *tarFile) readCloser() (io.ReadCloser, error) {
	f.m.Lock()
	if _, err := f.r.Seek(f.offset, io.SeekStart); err != nil {
		f.m.Unlock()
		return nil, err
	}
	close := func() error { f.m.Unlock(); return nil }
	return &readcloser{&io.LimitedReader{R: f.r, N: f.h.Size}, close}, nil
}
