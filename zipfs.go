package main

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"path"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
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

type dir interface {
	Name() string
	Files() []file
	Dirs() []dir
	AddDir(string) dir
	AddFile(file) file
	FindDir(string) dir
	FindFile(string) file
}

type plainDir struct {
	name string
	// NOTE: it could be good idea to change this to map[string]{file,dir} for
	// faster lookup
	files []file
	dirs  []dir
}

func (d *plainDir) Name() string {
	return d.name
}

func (d *plainDir) Files() []file {
	return d.files
}

func (d *plainDir) Dirs() []dir {
	return d.dirs
}

func (d *plainDir) FindDir(name string) dir {
	for _, dir := range d.dirs {
		if dir.Name() == name {
			return dir
		}
	}
	return nil
}

func (d *plainDir) FindFile(name string) file {
	for _, file := range d.files {
		if file.Name() == name {
			return file
		}
	}
	return nil
}

func (d *plainDir) AddFile(newFile file) file {
	for _, f := range d.files {
		if f.Name() == newFile.Name() {
			return f
		}
	}
	d.files = append(d.files, newFile)
	return newFile
}

func (d *plainDir) AddDir(name string) dir {
	existing := d.FindDir(name)
	if existing != nil {
		return existing
	}
	newDir := newPlainDir(name)
	d.dirs = append(d.dirs, newDir)
	return newDir
}

func (d *plainDir) String() string {
	return fmt.Sprintf("{dir name: '%s', files: '%v', dirs: '%v'}", d.Name(), d.files, d.dirs)
}

func newPlainDir(name string) dir {
	return &plainDir{name, nil, nil}
}

func recursiveAddDir(root dir, path string) dir {
	comps := strings.Split(path, "/")
	current := root
	for _, comp := range comps {
		if comp == "" {
			break
		}
		current = current.AddDir(comp)
	}
	return current
}

func recursiveFindDir(root dir, path string) dir {
	if root.Name() == path || path == "." {
		return root
	}

	comps := strings.Split(path, "/")
	current := root
	for _, comp := range comps {
		d := current.FindDir(comp)
		if d == nil {
			return nil
		}
		current = d
	}
	return current
}

func recursiveFindFile(root dir, p string) file {
	base := path.Dir(p)
	d := recursiveFindDir(root, base)
	if d == nil {
		return nil
	}
	return d.FindFile(path.Base(p))
}

// StaticTreeFs is a fuse filesystem that mounts tree like filesystem that do not
// change shape after mounting.
type StaticTreeFs struct {
	pathfs.FileSystem
	root dir
}

// NewZipFs returns new filesystem reading zip archive from r of size.
func NewZipFs(r io.ReaderAt, size int64) (pathfs.FileSystem, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	root := newPlainDir("")
	for _, f := range zipr.File {
		var file file
		switch {
		case strings.HasSuffix(f.Name, ".gz"):
			file = newGzipFile(f)
		case strings.HasSuffix(f.Name, ".xz"):
			file = newXzFile(f)
		case strings.HasSuffix(f.Name, ".bz2"):
			file = newBzip2File(f)
		default:
			file = newPlainFile(f)
		}
		// TODO: This probably should be done based on metadata from zip file
		// header.
		if f.Name[len(f.Name)-1] == '/' {
			recursiveAddDir(root, f.Name)
			continue
		}
		p := path.Dir(f.Name)
		d := root
		if p != "." {
			d = recursiveAddDir(root, p)
		}
		d.AddFile(file)
	}
	zfs := &StaticTreeFs{pathfs.NewDefaultFileSystem(), root}
	return pathfs.NewLockingFileSystem(zfs), nil
}

func (fs *StaticTreeFs) isDir(path string) bool {
	return recursiveFindDir(fs.root, path) != nil
}

func (fs *StaticTreeFs) fileSize(p string) (uint64, bool) {
	f := recursiveFindFile(fs.root, p)
	if f == nil {
		return 0, false
	}
	s, err := f.Size()
	if err != nil {
		debugf("file size failed for '%v': %v", p, err)
		return 0, false
	}
	return s, true
}

// OpenDir returns list of files and directories directly under path.
func (fs *StaticTreeFs) OpenDir(path string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	debugf("OpenDir: '%s'", path)
	d := recursiveFindDir(fs.root, path)
	if d == nil {
		return nil, fuse.ENOENT
	}
	tmp := make([]fuse.DirEntry, 0)
	for _, f := range d.Files() {
		tmp = append(tmp, fuse.DirEntry{Name: f.Name(), Mode: mode(true)})
	}
	for _, d := range d.Dirs() {
		tmp = append(tmp, fuse.DirEntry{Name: d.Name(), Mode: mode(false)})
	}
	return tmp, fuse.OK
}

// GetAttr returns attributes of path.
func (fs *StaticTreeFs) GetAttr(path string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	size, isFile := fs.fileSize(path)
	if !isFile && !fs.isDir(path) {
		debugf("GetAttr: '%s' -> does not exist", path)
		return nil, fuse.ENOENT
	}
	attr := &fuse.Attr{Mode: mode(isFile), Size: size}
	debugf("GetAttr: '%s' -> file:%v dir:%v (%v)", path, attr.IsRegular(), attr.IsDir(), attr)
	return attr, fuse.OK
}

// Open return File representing contents stored under path p.
func (fs *StaticTreeFs) Open(p string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	f := recursiveFindFile(fs.root, p)
	if f == nil {
		return nil, fuse.ENOENT
	}
	b, err := f.Bytes()
	if err != nil {
		debugf("open '%v' failed: %v", p, err)
		return nil, fuse.EIO
	}
	return nodefs.NewDataFile(b), fuse.OK
}

func mode(isFile bool) uint32 {
	if isFile {
		return uint32(0755) | fuse.S_IFREG
	}
	return uint32(0755) | fuse.S_IFDIR
}
