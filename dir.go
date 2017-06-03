package main

import (
	"fmt"
	"path"
	"strings"
)

type dir interface {
	setName(string)
	name() string
	files() []file
	dirs() []dir
	addEmptyDir(string) dir
	addDir(dir) dir
	setRecursiveDir(string, dir) bool
	addFile(file) file
	findDir(string) dir
	findFile(string) file
}

type plainDir struct {
	n string
	// NOTE: it could be good idea to change this to map[string]{file,dir} for
	// faster lookup
	f []file
	d []dir
}

func (d *plainDir) setName(name string) {
	d.n = name
}

func (d *plainDir) name() string {
	return d.n
}

func (d *plainDir) files() []file {
	return d.f
}

func (d *plainDir) dirs() []dir {
	return d.d
}

func (d *plainDir) findDir(name string) dir {
	for _, dir := range d.d {
		if dir.name() == name {
			return dir
		}
	}
	return nil
}

func (d *plainDir) findFile(name string) file {
	for _, file := range d.f {
		if file.name() == name {
			return file
		}
	}
	return nil
}

func (d *plainDir) addFile(newFile file) file {
	for _, f := range d.f {
		if f.name() == newFile.name() {
			return f
		}
	}
	d.f = append(d.f, newFile)
	return newFile
}

func (d *plainDir) addEmptyDir(name string) dir {
	existing := d.findDir(name)
	if existing != nil {
		return existing
	}
	newDir := newPlainDir(name)
	d.d = append(d.d, newDir)
	return newDir
}

func (d *plainDir) setRecursiveDir(name string, newDir dir) bool {
	parent := recursiveFindDir(d, path.Dir(name))
	debugf("'%v' parent: '%v': %v", name, path.Dir(name), parent)
	if parent == nil {
		return false
	}
	for _, e := range parent.dirs() {
		if e.name() == newDir.name() {
			return false
		}
	}
	parent.addDir(newDir)
	debugf("'%v' parent: '%v': %v", name, path.Dir(name), parent)
	return true
}

func (d *plainDir) addDir(newDir dir) dir {
	d.d = append(d.d, newDir)
	return newDir
}

func (d *plainDir) String() string {
	return fmt.Sprintf("{dir name: '%s', files: '%v', dirs: '%v'}", d.name(), d.f, d.d)
}

func newPlainDir(name string) dir {
	return &plainDir{name, nil, nil}
}

func recursiveAddDir(root dir, path string) dir {
	if path == "." {
		return root
	}
	comps := strings.Split(path, "/")
	current := root
	for _, comp := range comps {
		if comp == "" {
			break
		}
		current = current.addEmptyDir(comp)
	}
	return current
}

func unarchivedName(path string) string {
	if isArchive(path) {
		return removeExt(path)
	}
	return path
}
