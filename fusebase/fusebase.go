package fusebase

import (
	"github.com/samthor/fusebase/local"
	"github.com/zabawaba99/firego"

	"log"
	"time"
)

// FUSEBase provides a FUSE filesystem backed on Firebase.
type FUSEBase struct {
	f    *firego.Firebase
	root local.Node
}

// New creates a new FUSEBase based on the given firebase/key.
func New(firebase, key string) (*FUSEBase, error) {
	fb := &FUSEBase{}
	f := firego.New("https://"+firebase+".firebaseIO.com", nil)
	f.Auth(key)
	fb.f = f

	notifications := make(chan firego.Event)
	if err := f.Watch(notifications); err != nil {
		return nil, err
	}

	fb.root.Created = time.Now()

	go func() {
		defer f.StopWatching()
		for event := range notifications {
			now := time.Now()

			switch event.Type {
			case "put":
				err := fb.root.Handle(now, event.Path, event.Data)
				if err != nil {
					log.Fatalf("couldn't handle update: %v", err)
				}
			case "event_error":
				log.Printf("got firebase error: %+v", event.Data)
			default:
				log.Printf("ignoring unknown event: %#v", event)
			}
		}

		log.Fatalf("firebase API stopped")
	}()

	return fb, nil
}
