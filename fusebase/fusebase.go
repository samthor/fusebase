package fusebase

import (
	"github.com/samthor/fusebase/local"
	"github.com/zabawaba99/firego"

	"log"
	"time"
)

// FUSEBase provides a FUSE filesystem backed on Firebase.
type FUSEBase struct {
	fb *firego.Firebase
	fs *fsNode // fixed root node

	// TODO: nodes will become invalid when purged - then what?!
	nodes map[*local.Node]*fsNode
}

// New creates a new FUSEBase based on the given firebase/key.
func New(firebase, key string) (*FUSEBase, error) {
	fb := firego.New("https://"+firebase+".firebaseIO.com", nil)
	fb.Auth(key)

	notifications := make(chan firego.Event)
	if err := fb.Watch(notifications); err != nil {
		return nil, err
	}

	root := local.Node{
		Created: time.Now(),
		Key:     "", // intentionally not "/"
	}

	go func() {
		defer fb.StopWatching()
		for event := range notifications {
			now := time.Now()

			switch event.Type {
			case "put":
				err := root.Handle(now, event.Path, event.Data)
				if err != nil {
					log.Fatalf("couldn't handle update: %v", err)
				}
				log.Printf("update ok for: %v", event.Path)
			case "event_error":
				log.Printf("got firebase error: %+v", event.Data)
			default:
				log.Printf("ignoring unknown event: %#v", event)
			}
		}

		log.Fatalf("firebase API stopped")
	}()

	out := &FUSEBase{
		fb:    fb,
		nodes: make(map[*local.Node]*fsNode),
	}
	out.fs = &fsNode{Node: &root, f: out}
	return out, nil
}

func (f *FUSEBase) fbFor(key string) *firego.Firebase {
	if len(key) == 0 {
		return f.fb
	}
	if key[0] == '/' {
		return f.fb.Child(key)
	}
	return nil
}
