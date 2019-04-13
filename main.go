package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func debugf(format string, args ...interface{}) {
	if *verbose || *vverbose {
		log.Printf(format, args...)
	}
}

var (
	verbose  = flag.Bool("v", false, "verbose logging")
	vverbose = flag.Bool("vv", false, "very verbose logging")
)

func failOnErr(format string, err error) {
	if err != nil {
		log.Fatalf(format, err)
	}
}

func createDirIfNotPresent(dir string) error {
	return os.Mkdir(dir, 0777)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  snapshot_fs [OPTIONS] ARCHIVE MOUNTPOINT\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n\n")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nPlease report any bugs on https://github.com/tumdum/snapshot_fs")
	}
	flag.Parse()
	if len(flag.Args()) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	f, err := os.Open(flag.Arg(0))
	failOnErr("Could not open file: %v", err)

	stat, err := f.Stat()
	failOnErr("Could not stat file: %v", err)

	fs, err := newFsFromArchive(f, stat.Size(), flag.Arg(0))
	failOnErr("Could not parse archive: %v", err)

	if err := createDirIfNotPresent(flag.Arg(1)); err != nil && !os.IsExist(err) {
		failOnErr("Could not make "+flag.Arg(1)+": %v", err)
	} else if err == nil {
		defer os.Remove(flag.Arg(1))
	}

	nfs := pathfs.NewPathNodeFs(fs, nil)
	server, _, err := nodefs.MountRoot(flag.Arg(1), nfs.Root(), nil)
	failOnErr("Could not mount: %v", err)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		if err := server.Unmount(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unmount fs: %v", err)
			os.Exit(1)
		}
	}()

	server.SetDebug(*vverbose)
	log.Println("Serving")
	server.Serve()
}
