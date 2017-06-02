package main

import (
	"fmt"
	"io"
	p "path"
	"strings"
)

type DirReader interface {
	io.ReaderAt
	io.ReadSeeker
}

func isArchive(path string) bool {
	return strings.HasSuffix(path, ".zip") || strings.HasSuffix(path, ".tar")
}

func newDirFromArchive(r DirReader, size int64, path string) (dir, error) {
	if strings.HasSuffix(path, ".zip") {
		dir, err := newDirFromZip(r, size)
		if err != nil {
			return nil, err
		}
		return dir, nil
	} else if strings.HasSuffix(path, ".tar") {
		dir, err := newDirFromTar(r)
		if err != nil {
			return nil, err
		}
		return dir, nil
	}
	return nil, fmt.Errorf("unsupported archive format: %v", p.Ext(path))
}
