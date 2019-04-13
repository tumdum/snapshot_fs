package main

import (
	"compress/bzip2"
	"compress/gzip"
	"io"
	"io/ioutil"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

type file interface {
	name() string
	size() (uint64, error)
	modTime() time.Time
	readCloser() (io.ReadCloser, error)
}

func allBytes(f file) ([]byte, error) {
	rc, err := f.readCloser()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

func newGzipFile(f file) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }
	return &compressedFile{f, d, math.MaxUint64}
}

func newXzFile(f file) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return xz.NewReader(r) }
	return &compressedFile{f, d, math.MaxUint64}
}

func newBzip2File(f file) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }
	return &compressedFile{f, d, math.MaxUint64}
}

func removeExt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func isCompressed(path string) bool {
	pred := func(ext string) bool { return strings.HasSuffix(path, ext) }
	return pred(".gz") || pred(".xz") || pred(".bz2")
}

func uncompressedName(path string) string {
	if isCompressed(path) {
		return removeExt(path)
	}
	return path
}

func newFile(f file, ext string) file {
	switch ext {
	case ".gz":
		return newGzipFile(f)
	case ".xz":
		return newXzFile(f)
	case ".bz2":
		return newBzip2File(f)
	default:
		return f
	}
}

type compressedFile struct {
	file
	decompressor func(io.Reader) (io.Reader, error)
	s            uint64
}

func (f *compressedFile) name() string {
	return f.file.name()
}

type readcloser struct {
	io.Reader
	close func() error
}

func (r *readcloser) Close() error {
	return r.close()
}

func (f *compressedFile) readCloser() (io.ReadCloser, error) {
	r, err := f.file.readCloser()
	if err != nil {
		return nil, err
	}
	d, err := f.decompressor(r)
	if err != nil {
		r.Close()
		return nil, err
	}
	return &readcloser{d, r.Close}, nil
}

func (f *compressedFile) size() (uint64, error) {
	if f.s == math.MaxUint64 {
		b, err := allBytes(f)
		if err != nil {
			return 0, err
		}
		f.s = uint64(len(b))
	}
	return f.s, nil
}

func (f *compressedFile) String() string {
	return f.name()
}
