package main

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"io/ioutil"
	"math"
	"path"
	"strings"

	"github.com/ulikunitz/xz"
)

type file interface {
	Name() string
	Size() (uint64, error)
	Bytes() ([]byte, error)
}

type plainFile struct {
	z *zip.File
}

func (f *plainFile) Name() string {
	return path.Base(f.z.Name)
}

func (f *plainFile) Size() (uint64, error) {
	return f.z.UncompressedSize64, nil
}

func (f *plainFile) Bytes() ([]byte, error) {
	r, err := f.z.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

func (f *plainFile) String() string {
	return f.Name()
}

type compressedFile struct {
	plainFile
	decompressor func(io.Reader) (io.Reader, error)
	size         uint64
}

func (f *compressedFile) Name() string {
	return path.Base(f.z.Name)
}

func (f *compressedFile) Bytes() ([]byte, error) {
	r, err := f.z.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	c, err := f.decompressor(r)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(c)
}

func (f *compressedFile) Size() (uint64, error) {
	if f.size != math.MaxUint64 {
		return f.size, nil
	}
	b, err := f.Bytes()
	if err != nil {
		return 0, err
	}
	return uint64(len(b)), nil
}

func (f *compressedFile) String() string {
	return f.Name()
}

func newPlainFile(z *zip.File) *plainFile {
	return &plainFile{z}
}

func newGzipFile(z *zip.File) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }
	return &compressedFile{*newPlainFile(z), d, math.MaxUint64}
}

func newXzFile(z *zip.File) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return xz.NewReader(r) }
	return &compressedFile{*newPlainFile(z), d, math.MaxUint64}
}

func newBzip2File(z *zip.File) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }
	return &compressedFile{*newPlainFile(z), d, math.MaxUint64}
}

func newFile(f *zip.File) file {
	switch {
	case strings.HasSuffix(f.Name, ".gz"):
		return newGzipFile(f)
	case strings.HasSuffix(f.Name, ".xz"):
		return newXzFile(f)
	case strings.HasSuffix(f.Name, ".bz2"):
		return newBzip2File(f)
	default:
		return newPlainFile(f)
	}
}