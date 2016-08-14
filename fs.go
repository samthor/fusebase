package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/samthor/fusebase/fusebase"

	"flag"
	"log"
)

var (
	firebase = flag.String("firebase", "", "name of firebase")
	key      = flag.String("key", "", "database key from firebase")
	mount    = flag.String("mount", "", "mountpoint")
)

func main() {
	flag.Parse()
	if *firebase == "" {
		log.Fatalf("expected --firebase, none specified")
	}

	fb, err := fusebase.New(*firebase, *key)
	if err != nil {
		log.Fatalf("couldn't create fusebase: %v", err)
	}

	c, err := fuse.Mount(
		*mount,
		fuse.FSName("fusebase"),
		fuse.Subtype("fusebase"),
		fuse.LocalVolume(),
		fuse.AllowOther(),
	)
	if err != nil {
		log.Fatalf("couldn't mount fusebase: %v", err)
	}
	defer c.Close()

	err = fs.Serve(c, fb)
	<-c.Ready
	if c.MountError != nil {
		log.Fatal(c.MountError)
	}
}
