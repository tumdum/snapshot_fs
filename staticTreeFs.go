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
	dir
}

func (fs *StaticTreeFs) isDir(path string) bool {
	return recursiveFindDir(fs.dir, path) != nil
}

func (fs *StaticTreeFs) file(p string) file {
	return recursiveFindFile(fs.dir, p)
}

func (fs *StaticTreeFs) fileSize(p string) (uint64, bool) {
	f := fs.file(p)
	if f == nil {
		return 0, false
	}
	s, err := f.size()
	if err != nil {
		return 0, false
	}
	return s, true
}

// OpenDir returns list of files and directories directly under path.
func (fs *StaticTreeFs) OpenDir(path string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	debugf("OpenDir: '%s'", path)
	d := recursiveFindDir(fs.dir, path)
	if d == nil {
		return nil, fuse.ENOENT
	}
	entries := make([]fuse.DirEntry, 0)
	for _, f := range d.files() {
		entries = append(entries, fuse.DirEntry{Name: f.name(), Mode: mode(true)})
	}
	for _, d := range d.dirs() {
		entries = append(entries, fuse.DirEntry{Name: d.name(), Mode: mode(false)})
	}
	return entries, fuse.OK
}

// GetAttr returns attributes of path.
func (fs *StaticTreeFs) GetAttr(path string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f := fs.file(path)
	attr := &fuse.Attr{Mode: mode(f != nil)}
	if f == nil && !fs.isDir(path) {
		debugf("GetAttr: '%s' -> does not exist", path)
		return nil, fuse.ENOENT
	} else if f != nil {
		size, _ := f.size()
		attr.Size = size
		// Copy paste from https://github.com/hanwen/go-fuse/blob/master/zipfs/zipfs.go#L24
		attr.Mtime = uint64(f.modTime().Unix())
		attr.Atime, attr.Ctime = attr.Mtime, attr.Mtime
	}
	debugf("GetAttr: '%s' -> file:%v dir:%v (%v)", path, attr.IsRegular(), attr.IsDir(), attr)
	return attr, fuse.OK
}

// Open return File representing contents stored under path p.
func (fs *StaticTreeFs) Open(p string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	f := recursiveFindFile(fs.dir, p)
	if f == nil {
		return nil, fuse.ENOENT
	}
	b, err := allBytes(f)
	if err != nil {
		return nil, fuse.EIO
	}
	return nodefs.NewDataFile(b), fuse.OK
}

func recursiveFindDir(root dir, path string) dir {
	if root.name() == path || path == "." {
		return root
	}

	current := root
	for _, comp := range strings.Split(path, "/") {
		d := current.findDir(comp)
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
	return d.findFile(path.Base(p))
}

func mode(isFile bool) uint32 {
	if isFile {
		return uint32(0555) | fuse.S_IFREG
	}
	return uint32(0755) | fuse.S_IFDIR
}
