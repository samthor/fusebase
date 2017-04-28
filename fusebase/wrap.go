package fusebase

import (
	"runtime"
)

type fsWrap struct {
	*fsNode
}

// wrapNode wraps a *fsNode and a cleanup function to run when the *fsWrap is no longer referenced.
func wrapNode(node *fsNode, run func()) *fsWrap {
	ch := make(chan bool)
	wrap := &fsWrap{node}
	go func() {
		<-ch
		run()
	}()
	runtime.SetFinalizer(wrap, func(w *fsWrap) {
		close(ch)
	})
	return wrap
}
