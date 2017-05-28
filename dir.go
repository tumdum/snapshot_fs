package main

import (
	"fmt"
	"strings"
)

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
	if path == "." {
		return root
	}
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
