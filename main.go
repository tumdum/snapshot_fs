package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func debugf(format string, args ...interface{}) {
	if *verbose {
		log.Printf(format, args...)
	}
}

// ZipFs is a fuse filesystem that mounts zip archives
type ZipFs struct {
	pathfs.FileSystem
	z *zip.Reader
	// caching this way is fast enough for now. If there will be need to make
	// it faster, this could be chanegd to map from prefix to set of files.
	files map[string]*zip.File
}

func NewZipFs(r io.ReaderAt, size int64) (*ZipFs, error) {
	zipr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, f := range zipr.File {
		files[f.Name] = f
	}
	return &ZipFs{pathfs.NewDefaultFileSystem(), zipr, files}, nil
}

func (z *ZipFs) isFile(name string) bool {
	_, ok := z.files[name]
	return ok
}

func (z *ZipFs) fileSize(name string) (uint64, bool) {
	f, ok := z.files[name]
	if !ok {
		return 0, false
	}
	return f.UncompressedSize64, true
}

func (z *ZipFs) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	debugf("OpenDir: %s", name)
	files := make([]fuse.DirEntry, 0)
	seen := map[string]struct{}{}
	for _, entry := range z.z.File {
		if !strings.HasPrefix(entry.Name, name) {
			continue
		}
		suffix := strings.TrimPrefix(entry.Name, name)
		// There are only files in zip file. So OpenDir called on
		// file is an error.
		if suffix == "" {
			return nil, fuse.Status(syscall.ENOSYS)
		}

		components := strings.Split(suffix, "/")
		if components[0] == "" {
			components = components[1:]
		}
		// TODO: should I check len here?
		first := components[0]
		if _, ok := seen[first]; ok {
			continue
		}
		seen[first] = struct{}{}
		files = append(files, fuse.DirEntry{Name: first, Mode: z.mode(first)})
	}
	return files, 0
}

func (z *ZipFs) mode(name string) uint32 {
	mode := uint32(0755)
	if z.isFile(name) {
		mode |= fuse.S_IFREG
	} else {
		mode |= fuse.S_IFDIR
	}
	return mode
}

func (z *ZipFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	debugf("GetAttr: %s", name)
	size, _ := z.fileSize(name)
	return &fuse.Attr{Mode: z.mode(name), Size: size}, fuse.OK
}

func (z *ZipFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f, ok := z.files[name]
	if !ok {
		return nil, fuse.ENOENT
	}
	r, err := f.Open()
	if err != nil {
		debugf("failed to open '%v': %v", name, err)
		return nil, fuse.EIO // TODO: EIO?
	}
	defer r.Close()
	var b bytes.Buffer
	if _, err = io.Copy(&b, r); err != nil {
		debugf("faile to read '%v': %v", name, err)
		return nil, fuse.EIO
	}
	return nodefs.NewDataFile(b.Bytes()), fuse.OK
}

var verbose = flag.Bool("v", false, "verbose logging")

func main() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  hello MOUNTPOINT PATH_TO_ZIP")
	}

	f, err := os.Open(flag.Arg(1))
	if err != nil {
		log.Fatalf("Could not open zip file: %v", err)
	}
	stat, err := f.Stat()
	if err != nil {
		log.Fatalf("Could not stat zip file: %v", err)
	}

	fs, err := NewZipFs(f, stat.Size())
	if err != nil {
		log.Fatalf("Could not read zip file: %v", err)
	}

	nfs := pathfs.NewPathNodeFs(fs, nil)
	server, _, err := nodefs.MountRoot(flag.Arg(0), nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.SetDebug(*verbose)
	server.Serve()
}
