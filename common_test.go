package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"strings"
	"testing"

	"github.com/dsnet/compress/bzip2"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/ulikunitz/xz"
)

var (
	flatFile = map[string][]byte{
		"foo.txt": []byte("foo.txt file content"),
		"bar":     []byte("bar file content"),
		"empty":   []byte("empty"),
	}
	multiLevel = map[string][]byte{
		"a/b":     []byte("c"),
		"b":       []byte("d"),
		"e":       []byte("f"),
		"g/h/i/j": []byte("k"),
		"g/h/i/l": []byte("mmmmm"),
		"g/h/n":   []byte("o"),
		"g/hp":    []byte("r"),
	}
	multiLevelWithDirs = map[string][]byte{
		"a/":  []byte("dir"),
		"a/b": []byte("c"),
		"d/":  []byte("dir"),
	}
	withGziped = map[string][]byte{
		"a":         []byte("b"),
		"c.gz":      []byte("dddddddddddddddddddddddddddddddddddddddddddddddddddddd"),
		"f/g/h.gz":  []byte("iiiiii"),
		"f/g/j.txt": []byte("kkkkk"),
	}
	withXziped = map[string][]byte{
		"a":        []byte("b"),
		"c.xz":     []byte("dddddddddddddddddddddddddddddddddddddddddddddddddddddd"),
		"f/g/h.xz": []byte("iiiiii"),
		"f/g/j.xz": []byte("kkkkk"),
	}
	withBziped = map[string][]byte{
		"a":         []byte("b"),
		"c.bz2":     []byte("dddddddddddddddddddddddddddddddddddddddddddddddddddddd"),
		"f/g/h.bz2": []byte("iiiiii"),
		"f/g/j.bz2": []byte("kkkkk"),
	}
)

func mustNewDir(m map[string][]byte, typ string) dir {
	switch typ {
	case "zip":
		b := makeZipFileBytes(m)
		d, err := newDirFromArchive(bytes.NewReader(b), int64(len(b)), "archive.zip")
		if err != nil {
			panic(err)
		}
		return d
	case "tar":
		b := makeTarFile(m)
		d, err := newDirFromArchive(bytes.NewReader(b), int64(len(b)), "archive.tar")
		if err != nil {
			panic(err)
		}
		return d
	default:
		panic("unknown fs type: " + typ)
	}
}

func MustNewFs(m map[string][]byte, typ string) pathfs.FileSystem {
	return &StaticTreeFs{pathfs.NewDefaultFileSystem(), mustNewDir(m, typ)}
}

func mustPackGzip(content []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(content); err != nil {
		panic(err)
	}
	w.Close()
	return b.Bytes()
}

func mustPackXz(content []byte) []byte {
	var b bytes.Buffer
	w, err := xz.NewWriter(&b)
	if err != nil {
		panic(err)
	}
	if _, err := w.Write(content); err != nil {
		panic(err)
	}
	w.Close()
	return b.Bytes()
}

func mustPackBzip(content []byte) []byte {
	var b bytes.Buffer
	w, err := bzip2.NewWriter(&b, &bzip2.WriterConfig{Level: 3})
	if err != nil {
		panic(err)
	}
	if _, err := w.Write(content); err != nil {
		panic(err)
	}
	w.Close()
	return b.Bytes()
}

func mustMakeArchive(m map[string][]byte, typ string) []byte {
	switch typ {
	case "zip":
		return makeZipFileBytes(m)
	case "tar":
		return makeTarFile(m)
	default:
		panic("unknown fs type: " + typ)
	}
}

type brokenDirReader struct {
	dirReader
	error
	c, t int
}

func (r *brokenDirReader) Read(b []byte) (int, error) {
	if r.c == r.t {
		return 0, r.error
	}
	r.t++
	return r.dirReader.Read(b)
}

func (r *brokenDirReader) ReadAt(p []byte, off int64) (n int, err error) {
	if r.c == r.t {
		return 0, r.error
	}
	r.t++
	return r.dirReader.ReadAt(p, off)
}

func (r *brokenDirReader) Seek(offset int64, whence int) (int64, error) {
	if r.c == r.t {
		return 0, r.error
	}
	r.t++
	return r.dirReader.Seek(offset, whence)
}

func TestNewDirFromArchiveReturnsErrorOnBrokenReader(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		for i := 0; i != 100; i++ {
			b := mustMakeArchive(multiLevel, typ)
			br := bytes.NewReader(b)
			r := &brokenDirReader{br, errors.New("error"), i, 0}
			_, err := newDirFromArchive(r, int64(len(b)), typ)
			if err == nil {
				t.Fatalf("malformed archive did not generate error")
			}
		}
	}
}

func TestNewDirFromArchiveReturnsErrorOnMalformedInput(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		_, err := newDirFromArchive(strings.NewReader("malformed"), 3, "file."+typ)
		if err == nil {
			t.Fatalf("malformed archive did not generate error")
		}
	}
}

func TestNewDirFromArchiveOnUnsupportedFormatFails(t *testing.T) {
	_, err := newDirFromArchive(strings.NewReader("malformed"), 3, "file.foo")
	if err == nil {
		t.Fatalf("malformed archive did not generate error")
	}
}

func TestFsOpenDirOnEmptyFile(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(nil, typ)
		entries, status := fs.OpenDir("", &fuse.Context{})
		verifyStatus("", status, t)

		if len(entries) != 0 {
			t.Fatalf("Expected 0 entries, got %d: %v", len(entries), entries)
		}
	}
}

func TestFsOpenDirOnFlatFile(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(flatFile, typ)
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
}

func TestFsOpenDirOnFileInFlatFile(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(flatFile, typ)
		entries, status := fs.OpenDir("empty", &fuse.Context{})
		if status.Ok() {
			t.Fatalf("Expected error status, found ok")
		}
		if len(entries) != 0 {
			t.Fatalf("Expected no entries, found '%v'", entries)
		}
	}
}

func TestFsOpenOk(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		for _, config := range []map[string][]byte{multiLevel, withGziped, withXziped, withBziped} {
			fs := MustNewFs(config, typ)
			for name, content := range config {
				realName := uncompressedName(name)
				readContent := mustReadFuseFile(realName, len(content), fs, t)
				if readContent != string(content) {
					t.Fatalf("%v: Expected content of '%v' is '%v', got '%v'", typ, realName, content, readContent)
				}
			}
		}
	}
}

func TestFsGetAttrOk(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		for _, config := range []map[string][]byte{multiLevel, withGziped, withXziped, withBziped} {
			fs := MustNewFs(config, typ)
			for name, content := range config {
				realName := uncompressedName(name)
				attr, status := fs.GetAttr(realName, &fuse.Context{})
				verifyStatus(realName, status, t)
				if attr.Mode&fuse.S_IFREG == 0 {
					t.Fatalf("File '%v' is not a file", realName)
				}
				if uint64(len(content)) != attr.Size {
					t.Fatalf("File '%v' has size %d, but got %d", realName, len(content), attr.Size)
				}
			}
			_, status := fs.GetAttr("", &fuse.Context{})
			if !status.Ok() {
				t.Fatalf("Nok status for root of fs")
			}
		}
	}
}

func TestFsOpenDirOnMultiLevelFile(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevel, typ)
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
}

func TestFsOpenDirOnMultiLevelFileSubdir(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevel, typ)
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
}

func TestFsOpenDirModeMultiLevel(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevel, typ)
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
}

func TestFsOpenDirWithExplicitDirs(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevelWithDirs, typ)
		expected := map[string]struct{}{"b": {}}
		entries, status := fs.OpenDir("a", &fuse.Context{})
		verifyStatus("a", status, t)
		if len(expected) != len(entries) {
			t.Fatalf("Expected %d entries, got %d: %v vs %v", len(expected), len(entries), expected, entries)
		}
	}
}

func TestFsOpenDirNotExisting(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevel, typ)
		_, status := fs.OpenDir("aaaaaaaaaaaaaa", &fuse.Context{})
		if status.Ok() {
			t.Fatalf("Ok status returned for not existing directory")
		}
	}
}

func TestFsOpenNotExisting(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevel, typ)
		_, status := fs.Open("aaaaaaaaaaaaaa", 0, &fuse.Context{})
		if status.Ok() {
			t.Fatalf("Status Ok for not existing file, should be nok")
		}
	}
}

func TestFsGetAttrNok(t *testing.T) {
	for _, typ := range []string{"tar", "zip"} {
		fs := MustNewFs(multiLevel, typ)
		_, status := fs.GetAttr("aaaaaaaaaaaaaa", &fuse.Context{})
		if status.Ok() {
			t.Fatalf("Ok status returned for not existing path")
		}
	}
}
