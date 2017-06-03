package main

import (
	"fmt"
	"io"
	p "path"
	"strings"
)

type dirReader interface {
	io.ReaderAt
	io.ReadSeeker
}

func isArchive(path string) bool {
	return strings.HasSuffix(path, ".zip") || strings.HasSuffix(path, ".tar")
}

func newDirFromArchive(r dirReader, size int64, path string) (dir, error) {
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

func notCollidingCompressedName(path string, seen map[string]int) string {
	name := uncompressedName(path)
	if v, ok := seen[name]; ok && v > 1 {
		return path
	}
	return name
}

func notCollidingArchiveName(path string, seen map[string]int) string {
	name := unarchivedName(path)
	if v, ok := seen[name]; ok && v > 1 {
		return path
	}
	return name
}
