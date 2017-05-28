package main

import (
	"archive/zip"
	"io"
	"path"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// NewZipFs returns new filesystem reading zip archive from r of size.
func NewZipFs(r io.ReaderAt, size int64) (pathfs.FileSystem, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	root := newPlainDir("")
	for _, f := range zipr.File {
		file := newFile(f)
		// TODO: This probably should be done based on metadata from zip file
		// header.
		if f.Name[len(f.Name)-1] == '/' {
			recursiveAddDir(root, f.Name)
			continue
		}
		recursiveAddDir(root, path.Dir(f.Name)).AddFile(file)
	}
	zfs := &StaticTreeFs{pathfs.NewDefaultFileSystem(), root}
	return pathfs.NewLockingFileSystem(zfs), nil
}
