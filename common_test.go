package main

import (
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/pathfs"
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
