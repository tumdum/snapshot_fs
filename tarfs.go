package main

import (
	"archive/tar"
	"bytes"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type tarfs struct {
	dir
}

func findAllPathsInTar(r io.ReadSeeker) (map[string]int, error) {
	m := map[string]int{}
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
		name = uncompressedName(unarchivedName(name))
		m[name]++
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
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		offset, err := r.Seek(0, io.SeekCurrent)
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
				if err := addArchiveToTar(h, tr, d, seen); err != nil {
					return nil, err
				}
			} else {
				ext := path.Ext(h.Name)
				name := notCollidingCompressedName(h.Name, seen)
				d.addFile(newFile(&tarFile{h, name, r, m, offset}, ext))
			}
		}
	}
	return &tarfs{root}, nil
}

func addArchiveToTar(h *tar.Header, r io.Reader, parent dir, seen map[string]int) error {
	b := make([]byte, h.Size)
	if _, err := r.Read(b); err != nil && err != io.EOF {
		return err
	}
	tarDir, err := newDirFromArchive(bytes.NewReader(b), int64(len(b)), h.Name)
	if err != nil {
		return err
	}
	name := notCollidingArchiveName(h.Name, seen)
	tarDir.setName(path.Base(name))
	parent.addDir(tarDir)
	return nil
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

func (f *tarFile) modTime() time.Time {
	return f.h.ModTime
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
