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
	ReadCloser() (io.ReadCloser, error)
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

func (f *plainFile) ReadCloser() (io.ReadCloser, error) {
	return f.z.Open()
}

func (f *plainFile) Bytes() ([]byte, error) {
	r, err := f.ReadCloser()
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
	file
	decompressor func(io.Reader) (io.Reader, error)
	size         uint64
}

func (f *compressedFile) Name() string {
	return f.file.Name()
}

type readcloser struct {
	close func() error
	r     io.Reader
}

func (r *readcloser) Close() error {
	return r.close()
}

func (r *readcloser) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

func (f *compressedFile) ReadCloser() (io.ReadCloser, error) {
	r, err := f.file.ReadCloser()
	if err != nil {
		return nil, err
	}
	return &readcloser{r.Close, r}, nil
}

func (f *compressedFile) Bytes() ([]byte, error) {
	rc, err := f.ReadCloser()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	d, err := f.decompressor(rc)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(d)
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
	return &compressedFile{newPlainFile(z), d, math.MaxUint64}
}

func newXzFile(z *zip.File) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return xz.NewReader(r) }
	return &compressedFile{newPlainFile(z), d, math.MaxUint64}
}

func newBzip2File(z *zip.File) *compressedFile {
	d := func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil }
	return &compressedFile{newPlainFile(z), d, math.MaxUint64}
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
