package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/dsnet/compress/bzip2"
	"github.com/ulikunitz/xz"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func addFileToZipBytes(w *zip.Writer, path string, content []byte) {
	f, err := w.Create(path)
	if err != nil {
		panic(err)
	}
	var r io.Reader
	if strings.HasSuffix(path, ".gz") {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		if _, err := w.Write(content); err != nil {
			panic(err)
		}
		w.Close()
		r = &b
	} else if strings.HasSuffix(path, ".xz") {
		var b bytes.Buffer
		w, err := xz.NewWriter(&b)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(content); err != nil {
			panic(err)
		}
		w.Close()
		r = &b
	} else if strings.HasSuffix(path, ".bz2") {
		var b bytes.Buffer
		w, err := bzip2.NewWriter(&b, &bzip2.WriterConfig{Level: 3})
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(content); err != nil {
			panic(err)
		}
		w.Close()
		r = &b
	} else {
		r = bytes.NewReader(content)
	}
	if _, err = io.Copy(f, r); err != nil {
		panic(err)
	}
}

func addFileToZip(w *zip.Writer, path string, content []byte) {
	addFileToZipBytes(w, path, []byte(content))
}

func verifyStatus(path string, s fuse.Status, t *testing.T) {
	if !s.Ok() {
		panic("Status not ok for '" + path + "': " + s.String())
	}
}

func makeZipFile(files map[string]string) []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)

	for path, content := range files {
		addFileToZip(w, path, []byte(content))
	}

	w.Flush()
	w.Close()
	return b.Bytes()
}

func makeZipFileBytes(files map[string][]byte) []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)

	for path, content := range files {
		addFileToZip(w, path, content)
	}

	w.Flush()
	w.Close()
	return b.Bytes()
}

var (
	flatFile = map[string]string{
		"foo.txt": "foo.txt file content",
		"bar":     "bar file content",
		"empty":   "empty",
	}
	multiLevel = map[string][]byte{
		"a/b":     []byte("c"),
		"b":       []byte("d"),
		"e":       []byte("f"),
		"g/h/i/j": []byte("k"),
		"g/h/i/l": []byte("m"),
		"g/h/n":   []byte("o"),
		"g/hp":    []byte("r"),
	}
	multiLevelWithZip = map[string][]byte{
		"a/d.zip": makeZipFileBytes(multiLevel),
		"e":       []byte("f"),
	}
	multiLevelWithDirs = map[string]string{
		"a/":  "",
		"a/b": "c",
	}
	withGziped = map[string]string{
		"a":         "b",
		"c.gz":      "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		"f/g/h.gz":  "iiiiii",
		"f/g/j.txt": "kkkkk",
	}
	withXziped = map[string]string{
		"a":        "b",
		"c.xz":     "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		"f/g/h.xz": "iiiiii",
		"f/g/j.xz": "kkkkk",
	}
	withBziped = map[string]string{
		"a":         "b",
		"c.bz2":     "dddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		"f/g/h.bz2": "iiiiii",
		"f/g/j.bz2": "kkkkk",
	}
)

func keys(m map[string]string) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func MustNewZipFs(b []byte) pathfs.FileSystem {
	r := bytes.NewReader(b)
	fs, err := NewZipFs(r, int64(r.Len()))
	if err != nil {
		panic(err)
	}
	return fs
}

func TestNewZipFsReturnsErrorOnMalformedZipArchive(t *testing.T) {
	r := strings.NewReader("test")
	_, err := NewZipFs(r, 4)
	if err == nil {
		t.Fatalf("Passing malformed reader to NewZipFs did not result in error")
	}
}

func TestZipFsOpenDirOnEmptyFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(nil))
	entries, status := fs.OpenDir("", &fuse.Context{})
	verifyStatus("", status, t)

	if len(entries) != 0 {
		t.Fatalf("Expected 0 entries, got %d: %v", len(entries), entries)
	}
}

func TestZipFsOpenDirOnFlatFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(flatFile))
	entries, status := fs.OpenDir("", &fuse.Context{})
	verifyStatus("", status, t)

	if len(entries) != len(flatFile) {
		t.Fatalf("Expected %d entries, got %d: %v vs %v", len(flatFile), len(entries), keys(flatFile), entries)
	}
	for _, entry := range entries {
		if _, ok := flatFile[entry.Name]; !ok {
			t.Fatalf("Found unexpected name '%v'", entry.Name)
		}
	}
}

func TestZipFsOpenDirOnFileInFlatFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(flatFile))
	entries, status := fs.OpenDir("empty", &fuse.Context{})
	if status.Ok() {
		t.Fatalf("Expected error status, found ok")
	}
	if len(entries) != 0 {
		t.Fatalf("Expected no entries, found '%v'", entries)
	}
}

func TestZipFsOpenDirOnMultiLevelFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevel))
	entries, status := fs.OpenDir("", &fuse.Context{})
	verifyStatus("", status, t)

	// name -> isFile
	expected := map[string]bool{
		"a": false,
		"b": true,
		"e": true,
		"g": false,
	}
	if len(entries) != len(expected) {
		t.Fatalf("Expected %d entries, got %d: %v", len(expected), len(entries), entries)
	}
	for _, entry := range entries {
		isFile, ok := expected[entry.Name]
		if !ok {
			t.Fatalf("Found unexpected name '%v'", entry.Name)
		}
		if (entry.Mode&fuse.S_IFREG != 0) != isFile {
			t.Fatalf("File '%v' is not a file in listing", entry.Name)
		}
	}
}

func TestZipFsOpenDirOnMultiLevelFileSubdir(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevel))
	entries, status := fs.OpenDir("g/h", &fuse.Context{})
	verifyStatus("g/h", status, t)

	// name -> isFile
	expected := map[string]bool{
		"i": false,
		"n": true,
	}

	if len(entries) != len(expected) {
		t.Fatalf("Expected %d entries, got %d: %v", len(expected), len(entries), entries)
	}

	for _, entry := range entries {
		isFile, ok := expected[entry.Name]
		if !ok {
			t.Fatalf("Found unexpected name '%v'", entry.Name)
		}
		if (entry.Mode&fuse.S_IFREG != 0) != isFile {
			t.Fatalf("File '%v' is not a file in listing", entry.Name)
		}
	}
}

func TestZipFsOpenDirModeMultiLevel(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevel))
	entries, _ := fs.OpenDir("", &fuse.Context{})
	for _, entry := range entries {
		_, isFile := multiLevel[entry.Name]
		if isFile && (entry.Mode&fuse.S_IFREG == 0) {
			t.Fatalf("File '%v' is not a file", entry.Name)
		} else if !isFile && (entry.Mode&fuse.S_IFDIR == 0) {
			t.Fatalf("Dir '%v' is not a dir", entry.Name)
		}
	}
	entries, _ = fs.OpenDir("g/h", &fuse.Context{})
	for _, entry := range entries {
		_, isFile := multiLevel["g/h/"+entry.Name]
		if isFile && (entry.Mode&fuse.S_IFREG == 0) {
			t.Fatalf("File '%v' is not a file", entry.Name)
		} else if !isFile && (entry.Mode&fuse.S_IFDIR == 0) {
			t.Fatalf("Dir '%v' is not a dir", entry.Name)
		}
	}
}

func TestZipFsOpenDirWithExplicitDirs(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(multiLevelWithDirs))
	expected := map[string]struct{}{"b": {}}
	entries, status := fs.OpenDir("a", &fuse.Context{})
	verifyStatus("a", status, t)
	if len(expected) != len(entries) {
		t.Fatalf("Expected %d entries, got %d: %v vs %v", len(expected), len(entries), expected, entries)
	}
}

func TestZipFsOpenDirNotExisting(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevel))
	_, status := fs.OpenDir("aaaaaaaaaaaaaa", &fuse.Context{})
	if status.Ok() {
		t.Fatalf("Ok status returned for not existing directory")
	}
}

func mustReadFuseFile(name string, l int, fs pathfs.FileSystem, t *testing.T) string {
	f, status := fs.Open(name, 0, &fuse.Context{})
	verifyStatus(name, status, t)
	b := make([]byte, l)
	readResult, status := f.Read(b, 0)
	verifyStatus(name, status, t)
	content, status := readResult.Bytes(b)
	verifyStatus(name, status, t)
	return string(content)
}

func TestZipFsOpenNotExisting(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevel))
	_, status := fs.Open("aaaaaaaaaaaaaa", 0, &fuse.Context{})
	if status.Ok() {
		t.Fatalf("Status Ok for not existing file, should be nok")
	}
}

func TestZipFsOpenOk(t *testing.T) {
	for _, config := range []map[string]string{ /*multiLevel, */ withGziped, withXziped, withBziped} {
		fs := MustNewZipFs(makeZipFile(config))
		for name, content := range config {
			readContent := mustReadFuseFile(name, len(content), fs, t)
			if readContent != content {
				t.Fatalf("Expected content of '%v' is '%v', got '%v'", name, content, readContent)
			}
		}
	}
}

func TestZipFsAccessingMalformedCompressed(t *testing.T) {
	files := map[string][]byte{
		"foo.gz": []byte("malformed"),
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for path, content := range files {
		f, err := w.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		r := bytes.NewReader(content)
		if _, err = io.Copy(f, r); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	w.Close()
	fs := MustNewZipFs(b.Bytes())
	_, status := fs.Open("foo.gz", 0, &fuse.Context{})
	if status.Ok() {
		t.Fatalf("Opening malformed gz file did not fail")
	}
	_, status = fs.GetAttr("foo.gz", &fuse.Context{})
	if status.Ok() {
		t.Fatalf("GetAttr malformed gz file did not fail")
	}
}

func TestZipFsGetAttrOk(t *testing.T) {
	for _, config := range []map[string]string{ /*multiLevel, */ withGziped, withXziped, withBziped} {
		fs := MustNewZipFs(makeZipFile(config))
		for name, content := range config {
			attr, status := fs.GetAttr(name, &fuse.Context{})
			verifyStatus(name, status, t)
			if attr.Mode&fuse.S_IFREG == 0 {
				t.Fatalf("File '%v' is not a file", name)
			}
			if uint64(len(content)) != attr.Size {
				t.Fatalf("File '%v' has size %d, but got %d", name, len(content), attr.Size)
			}
		}
		_, status := fs.GetAttr("", &fuse.Context{})
		if !status.Ok() {
			t.Fatalf("Nok status for root of fs")
		}
	}
}

func TestZipFsGetAttrNok(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevel))
	_, status := fs.GetAttr("aaaaaaaaaaaaaa", &fuse.Context{})
	if status.Ok() {
		t.Fatalf("Ok status returned for not existing path")
	}
}

func TestZipFsGetAttrOfZip(t *testing.T) {
	fs := MustNewZipFs(makeZipFileBytes(multiLevelWithZip))
	attr, status := fs.GetAttr("a/d.zip", &fuse.Context{})
	verifyStatus("a/d.zip", status, t)
	if attr.Mode&fuse.S_IFDIR == 0 {
		t.Fatalf("'a/d.zip' should be dir, but is not")
	}
	_, status = fs.GetAttr("a/d.zip/b", &fuse.Context{})
	verifyStatus("a/d.zip/b", status, t)
}

func verifyDirName(d dir, name string, t *testing.T) {
	if d.name() != name {
		t.Fatalf("Expected name '%v', got '%v'", name, d.name())
	}
}

func TestAddDir(t *testing.T) {
	root := newPlainDir("")
	ret := recursiveAddDir(root, "foo/bar/baz")
	verifyDirName(ret, "baz", t)

	foo := root.findDir("foo")
	verifyDirName(foo, "foo", t)
	verifyDirName(recursiveFindDir(root, "foo"), "foo", t)

	bar := foo.findDir("bar")
	verifyDirName(bar, "bar", t)
	verifyDirName(recursiveFindDir(root, "foo/bar"), "bar", t)

	baz := bar.findDir("baz")
	verifyDirName(baz, "baz", t)
	verifyDirName(recursiveFindDir(root, "foo/bar/baz"), "baz", t)
}
