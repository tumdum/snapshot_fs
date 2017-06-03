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
		// TODO: This probably should be done based on metadata from zip file
		// header.
		if f.Name[len(f.Name)-1] == '/' {
			recursiveAddDir(root, f.Name)
			continue
		}
		if isArchive(f.Name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			b, err := ioutil.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			br := bytes.NewReader(b)
			dir, err := newDirFromArchive(br, int64(len(b)), f.Name)
			if err != nil {
				return nil, err
			}
			name := notCollidingArchiveName(f.Name, seen)
			dir.setName(path.Base(name))
			d := recursiveAddDir(root, path.Dir(f.Name))
			if d.addDir(dir) == nil {
				return nil, fmt.Errorf("failed to add fs under '%v'", name)
			}
			continue
		}
		recursiveAddDir(root, path.Dir(f.Name)).addFile(file)
	}
	return root, nil
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
