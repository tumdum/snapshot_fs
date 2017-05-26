package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func debugf(format string, args ...interface{}) {
	if *verbose {
		log.Printf(format, args...)
	}
}

func isGzip(path string) bool {
	return strings.HasSuffix(path, ".gz")
}

func isXz(path string) bool {
	return strings.HasSuffix(path, ".xz")
}

func isBzip(path string) bool {
	return strings.HasSuffix(path, ".bz2")
}

func isUncompressed(path string) bool {
	return true
}

var verbose = flag.Bool("v", false, "verbose logging")

func failOnErr(format string, err error) {
	if err != nil {
		log.Fatalf(format, err)
	}
}

func main() {
	flag.Parse()
	if len(flag.Args()) < 2 {
		log.Fatal("Usage:\n  snapshot_fs MOUNTPOINT PATH_TO_ZIP")
	}

	f, err := os.Open(flag.Arg(1))
	failOnErr("Could not opne zip file: %v", err)

	stat, err := f.Stat()
	failOnErr("Could not stat zip file: %v", err)

	fs, err := NewZipFs(f, stat.Size())
	failOnErr("Could not read zip file: %v", err)

	nfs := pathfs.NewPathNodeFs(fs, nil)
	server, _, err := nodefs.MountRoot(flag.Arg(0), nfs.Root(), nil)
	failOnErr("Could not mount: %v", err)

	server.SetDebug(*verbose)
	server.Serve()
}
