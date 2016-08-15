Quick run instructions-

```bash
$ go get -u github.com/samthor/fusebase
$ cd ~/path/to/goroot/github.com/samthor/fusebase
$ go run fs.go --firebase=name-of-firebase --key=legacy-database-key --mount=/mnt/fusebase
```

Uses [bazil/fuse](https://github.com/bazil/fuse) (although should switch to [go-fuse](https://github.com/hanwen/go-fuse)) and [firego](https://github.com/zabawaba99/firego). 
