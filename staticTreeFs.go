package main

import (
	"path"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// StaticTreeFs is a fuse filesystem that mounts tree like filesystem that do not
// change shape after mounting.
type StaticTreeFs struct {
	pathfs.FileSystem
	root dir
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
	entries := make([]fuse.DirEntry, 0)
	for _, f := range d.Files() {
		entries = append(entries, fuse.DirEntry{Name: f.Name(), Mode: mode(true)})
	}
	for _, d := range d.Dirs() {
		entries = append(entries, fuse.DirEntry{Name: d.Name(), Mode: mode(false)})
	}
	return entries, fuse.OK
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

func recursiveFindDir(root dir, path string) dir {
	if root.Name() == path || path == "." {
		return root
	}

	current := root
	for _, comp := range strings.Split(path, "/") {
		d := current.FindDir(comp)
		if d == nil {
			return nil
		}
		current = d
	}
	return current
}

func recursiveFindFile(root dir, p string) file {
	d := recursiveFindDir(root, path.Dir(p))
	if d == nil {
		return nil
	}
	return d.FindFile(path.Base(p))
}

func mode(isFile bool) uint32 {
	if isFile {
		return uint32(0755) | fuse.S_IFREG
	}
	return uint32(0755) | fuse.S_IFDIR
}