[![Build Status](https://travis-ci.org/tumdum/snapshot_fs.svg?branch=go)](https://travis-ci.org/tumdum/snapshot_fs)
[![Coverage Status](https://coveralls.io/repos/github/tumdum/snapshot_fs/badge.svg?branch=master)](https://coveralls.io/github/tumdum/snapshot_fs?branch=master)
[![Coverage Status](https://goreportcard.com/badge/github.com/tumdum/snapshot_fs)](https://goreportcard.com/report/github.com/tumdum/snapshot_fs)
# snapshot_fs
read only fuse filesystem for mounting archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives containing archives

WIP.

To mount tar (or zip) archive execute:
```
$ go get github.com/tumdum/snapshot_fs
$ go install github.com/tumdum/snapshot_fs
$ snapshot_fs foo gutenberg/gutenberg.tar &
$ tree foo
foo
├── 111.txt
├── 151.txt
├── 171.txt
├── 201.txt
├── 222.txt
├── 243.txt
├── 271.txt
├── 297.txt
└── gutenberg2
    ├── 114.txt
    ├── 150.txt
    ├── 172.txt
    ├── 252.txt
    ├── 288.txt
    ├── gutenberg.tar
    │   ├── 100.txt
    │   ├── 160.txt
    │   ├── 161.txt
    │   ├── 207.txt
    │   ├── 211.txt
    │   ├── 249.txt
    │   ├── 250.txt
    │   ├── 250.txt.gz
    │   ├── 251.txt
    │   └── 300.txt
    └── gutenberg.zip
        ├── 101.txt
        ├── 156.txt
        ├── 162.txt
        ├── 210.txt.bz2
        ├── 210.txt.gz
        ├── 246.txt
        ├── 254.txt
        └── 299.txt
```
