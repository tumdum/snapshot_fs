package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

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

func main() {
	flag.Parse()
	if len(flag.Args()) < 2 {
		log.Fatal("Usage:\n  snapshot_fs MOUNTPOINT ARCHIVE")
	}

	f, err := os.Open(flag.Arg(1))
	failOnErr("Could not opne file: %v", err)

	stat, err := f.Stat()
	failOnErr("Could not stat file: %v", err)

	var fs pathfs.FileSystem
	if strings.HasSuffix(flag.Arg(1), ".zip") {
		tmp, err := NewZipFs(f, stat.Size())
		failOnErr("Could not read zip file: %v", err)
		fs = tmp
	} else if strings.HasSuffix(flag.Arg(1), ".tar") {
		tmp, err := newTarFs(f)
		failOnErr("Could not read tar file: %v", err)
		fs = tmp
	}

	nfs := pathfs.NewPathNodeFs(fs, nil)
	server, _, err := nodefs.MountRoot(flag.Arg(0), nfs.Root(), nil)
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
