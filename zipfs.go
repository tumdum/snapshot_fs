package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func findAllPathsInZip(z *zip.Reader) map[string]int {
	m := map[string]int{}
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		name = uncompressedName(unarchivedName(name))
		m[name]++
	}
	return m
}

func newDirFromZip(r io.ReaderAt, size int64) (dir, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	seen := findAllPathsInZip(zipr)
	root := newPlainDir("")
	for _, f := range zipr.File {
		ext := path.Ext(f.Name)
		name := notCollidingCompressedName(f.Name, seen)
		file := newFile(newZipFile(f, name), ext)
		parent := recursiveAddDir(root, path.Dir(f.Name))
		if isArchive(f.Name) {
			if err := addArchiveToZip(f, parent, seen); err != nil {
				return nil, err
			}
		} else if f.Name[len(f.Name)-1] != '/' {
			parent.addFile(file)
		}
	}
	return root, nil
}

func addArchiveToZip(f *zip.File, parent dir, seen map[string]int) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	b, err := ioutil.ReadAll(rc)
	if err != nil {
		return err
	}
	br := bytes.NewReader(b)
	dir, err := newDirFromArchive(br, int64(len(b)), f.Name)
	if err != nil {
		return err
	}
	name := notCollidingArchiveName(f.Name, seen)
	dir.setName(path.Base(name))
	if parent.addDir(dir) == nil {
		return fmt.Errorf("failed to add fs under '%v'", name)
	}
	return nil
}

func newStaticTreeFsFromZip(r io.ReaderAt, size int64) (*StaticTreeFs, error) {
	root, err := newDirFromZip(r, size)
	if err != nil {
		return nil, err
	}
	return &StaticTreeFs{pathfs.NewDefaultFileSystem(), root}, nil
}

// NewZipFs returns new filesystem reading zip archive from r of size.
func NewZipFs(r io.ReaderAt, size int64) (pathfs.FileSystem, error) {
	zfs, err := newStaticTreeFsFromZip(r, size)
	if err != nil {
		return nil, err
	}
	return pathfs.NewLockingFileSystem(zfs), nil
}

type zipFile struct {
	*zip.File
	n string
}

func (f *zipFile) name() string {
	return path.Base(f.n)
}

func (f *zipFile) size() (uint64, error) {
	return f.UncompressedSize64, nil
}

func (f *zipFile) readCloser() (io.ReadCloser, error) {
	return f.Open()
}

func (f *zipFile) String() string {
	return f.name()
}

func newZipFile(z *zip.File, name string) file {
	return &zipFile{z, name}
}
