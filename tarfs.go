package main

import (
	"archive/tar"
	"io"
	"path"
)

type tarfs struct {
	root dir
}

func (t *tarfs) setName(name string) {
	t.root.setName(name)
}
func (t *tarfs) name() string { return t.root.name() }
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

func NewTarFs(r io.Reader) (dir, error) {
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
		base := path.Dir(h.Name)
		d := recursiveAddDir(root, base)
		d.addFile(&tarfile{h})
	}
	return &tarfs{root}, nil
}

type tarfile struct {
	h *tar.Header
}

func (f *tarfile) name() string                       { return path.Base(f.h.Name) }
func (f *tarfile) size() (uint64, error)              { return 0, nil }
func (f *tarfile) readCloser() (io.ReadCloser, error) { return nil, nil }
