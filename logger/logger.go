package main

import (
	"github.com/samthor/fusebase/local"
	"github.com/zabawaba99/firego"

	"flag"
	"fmt"
	"log"
	"time"
)

var (
	firebase = flag.String("firebase", "", "name of firebase")
	key      = flag.String("key", "", "database key from firebase")
)

func main() {
	flag.Parse()
	if *firebase == "" {
		log.Fatalf("expected --firebase, none specified")
	}

	f := firego.New("https://"+*firebase+".firebaseIO.com", nil)
	f.Auth(*key)

	notifications := make(chan firego.Event)
	if err := f.Watch(notifications); err != nil {
		log.Fatal(err)
	}

	me := &local.Node{Created: time.Now()}

	defer f.StopWatching()
	for event := range notifications {
		switch event.Type {
		case "put":
			me.Handle(time.Now(), event.Path, event.Data)
			me.Print()
			fmt.Println("")
		case "event_error":
			log.Printf("got firebase error: %+v", event.Data)
		default:
			log.Printf("ignoring unknown event: %#v", event)
		}
	}
	fmt.Printf("Notifications have stopped\n")
}
