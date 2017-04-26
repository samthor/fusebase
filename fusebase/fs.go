package fusebase

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/samthor/fusebase/local"

	"log"
	"os"
	"time"

	"golang.org/x/net/context"
)

// TODO: None of this is threadsafe. Updates come in on their own goroutine,
// and it's unclear where bazil.org/fuse library calls these handlers from.

func (f *FUSEBase) Root() (fs.Node, error) {
	return f.fs, nil
}

type fsNode struct {
	Node *local.Node
	f    *FUSEBase
	b    []byte // cache
}

func (node *fsNode) isDir() bool {
	return node.Node.Map() != nil
}

func (node *fsNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = node.Node.Inode
	if node.isDir() {
		a.Mode = os.ModeDir | 0555
	} else {
		node.b = node.Node.Bytes()
		a.Size = uint64(len(node.b))
		a.Mode = 0644
	}
	return nil
}

func (node *fsNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	m := node.Node.Map()
	sub := m[name]
	if m == nil || sub == nil {
		// does not exist or not a dir
		return nil, fuse.ENOENT
	}

	// TODO: return the same fsNode for the same local.Node.
	out := &fsNode{Node: sub, f: node.f}
	wrap, ch := wrapNode(out)
	go func() {
		<-ch
		log.Printf("node closed: %v", out.Node.Key)
	}()

	return wrap, nil
}

func (node *fsNode) internalCreate(ctx context.Context, name string, dir bool) (fs.Node, error) {
	m := node.Node.Map()
	if m == nil {
		// not a directory
		return nil, fuse.ENOENT
	}

	sub := m[name]
	if sub != nil {
		// can't create a file twice
		return nil, fuse.EIO
	}

	var value interface{}
	if dir {
		value = make(map[string]interface{})
	} else {
		value = ""
	}

	// TODO: this doesn't push to Firebase until something gets written to it; the node is transient
	err := node.Node.Handle(time.Now(), "/"+name, value)
	if err != nil {
		log.Printf("couldn't create new path (path=%v, dir=%v): %v", name, dir, err)
		return nil, fuse.EIO
	}
	return node.Lookup(ctx, name)
}

func (node *fsNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	n, err := node.internalCreate(ctx, req.Name, false)
	return n, n, err
}

func (node *fsNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	return node.internalCreate(ctx, req.Name, true)
}

func (node *fsNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	// TODO: same sync write as below
	key := node.Node.Key + "/" + req.Name
	fb := node.f.fbFor(key)
	if fb == nil {
		return fuse.EIO
	}

	// this works for directories too, so rmdir a directory removes all its children
	out := fb.Set(nil)
	if out == nil {
		// TODO: purge?
	}
	return out
}

func (node *fsNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	m := node.Node.Map()
	if m == nil {
		// not a directory
		return nil, fuse.EIO
	}

	out := make([]fuse.Dirent, 0, len(m))
	for k, v := range m {
		ent := fuse.Dirent{Inode: v.Inode, Name: k}
		if v.Map() != nil {
			ent.Type = fuse.DT_Dir
		} else {
			ent.Type = fuse.DT_File
		}
		out = append(out, ent)
	}
	return out, nil
}

func (node *fsNode) ReadAll(ctx context.Context) ([]byte, error) {
	return node.b, nil
}

func (node *fsNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if node.isDir() {
		return fuse.EIO
	}
	if req.Offset != 0 && int(req.Offset) < len(node.b) {
		// allow write at zero, _or_ at end (replaces value)
		return fuse.EIO
	}
	if node.Node.Data == nil {
		// for some reason this file has no data
		log.Printf("set with nil existing data: %v", node.Node.Key)
		return fuse.EIO
	}

	value := bytesToValue(req.Data, node.Node.Data)
	resp.Size = len(req.Data)

	// TODO: This is a synchronous write that doesn't update the local state. We probably want to
	// optimistically set locally and send the request later (support offline), then wait until the
	// update comes in (during the write, or immediately after it).
	log.Printf("set: %v => %v", node.Node.Key, value)
	fb := node.f.fbFor(node.Node.Key)
	if fb == nil {
		return fuse.EIO
	}
	return fb.Set(value) // set, because this is a file node
}
