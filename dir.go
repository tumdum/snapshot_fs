package main

import (
	"fmt"
	"strings"
)

type dir interface {
	setName(string)
	name() string
	files() []file
	dirs() []dir
	addEmptyDir(string) dir
	addDir(dir) dir
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
	for _, d := range d.d {
		if d.name() == newFile.name() {
			return nil
		}
	}
	d.f = append(d.f, newFile)
	return newFile
}

func (d *plainDir) addEmptyDir(name string) dir {
	return d.addDir(newPlainDir(name))
}

func (d *plainDir) addDir(newDir dir) dir {
	existing := d.findDir(newDir.name())
	if existing != nil {
		return existing
	}
	for i := range d.f {
		if d.f[i].name() == newDir.name() {
			d.f = append(d.f[:i], d.f[i+1:]...)
			break
		}
	}
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
