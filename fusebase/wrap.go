package fusebase

import (
	"runtime"
)

type fsWrap struct {
	*fsNode
}

// wrapNode wraps a *fsNode, returning a *fsWrap and a channel that will be closed when the *fsWrap
// is no longer referenced (finalized).
func wrapNode(node *fsNode) (wrap *fsWrap, ch chan bool) {
	ch = make(chan bool)
	wrap = &fsWrap{node}
	runtime.SetFinalizer(wrap, func(w *fsWrap) {
		close(ch)
	})
	return
}
