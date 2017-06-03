package main

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/dsnet/compress/bzip2"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/ulikunitz/xz"
)

func MustNewFs(m map[string][]byte, fsType string) pathfs.FileSystem {
	switch fsType {
	case "zip":
		return MustNewZipFs(makeZipFileBytes(m))
	case "tar":
		return MustNewTarFs(makeTarFile(m))
	default:
		panic("unknown fs type: " + fsType)
	}
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
				readContent := mustReadFuseFile(name, len(content), fs, t)
				if readContent != string(content) {
					t.Fatalf("Expected content of '%v' is '%v', got '%v'", name, content, readContent)
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
