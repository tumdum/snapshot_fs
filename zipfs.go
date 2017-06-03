package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func newDirFromZip(r io.ReaderAt, size int64) (dir, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	root := newPlainDir("")
	for _, f := range zipr.File {
		file := newFile(newZipFile(f))
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
			dir.setName(path.Base(f.Name))
			recursiveAddDir(root, path.Dir(f.Name))
			if !root.setRecursiveDir(f.Name, dir) {
				return nil, fmt.Errorf("failed to add fs under '%v'", f.Name)
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
	z *zip.File
}

func (f *zipFile) name() string {
	return path.Base(f.z.Name)
}

func (f *zipFile) size() (uint64, error) {
	return f.z.UncompressedSize64, nil
}

func (f *zipFile) readCloser() (io.ReadCloser, error) {
	return f.z.Open()
}

func (f *zipFile) String() string {
	return f.name()
}

func newZipFile(z *zip.File) file {
	return &zipFile{z}
}
