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
	return &fsNode{Node: &f.root, f: f}, nil
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
	if m == nil {
		// not a directory
		return nil, fuse.ENOENT
	}

	sub := m[name]
	if sub == nil {
		return nil, fuse.ENOENT
	}
	return &fsNode{Node: sub, f: node.f}, nil
}

func (node *fsNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	m := node.Node.Map()
	if m == nil {
		// not a directory
		return []fuse.Dirent{}, nil
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

func (node *fsNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	m := node.Node.Map()
	if m == nil {
		// not a directory
		return nil, nil, fuse.ENOENT
	}

	sub := m[req.Name]
	if sub != nil {
		// can't create a file twice
		return nil, nil, fuse.EIO
	}

	// TODO: this doesn't push to Firebase until something gets written to it
	err := node.Node.Handle(time.Now(), "/" + req.Name, "")
	if err != nil {
		log.Printf("couldn't create new path (path=%v): %v", req.Name, err)
		return nil, nil, fuse.EIO
	}

	n, err := node.Lookup(ctx, req.Name)
	return n, n, err
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

	// TODO: This is a synchronous write that doesn't update the local state. We probably want to
	// optimistically set locally and send the request later (support offline), then wait until the
	// update comes in (during the write, or immediately after it).
	log.Printf("set: %v => %v", node.Node.Key, value)
	fb := node.f.f
	if node.Node.Key != "" {
		fb = fb.Child(node.Node.Key)
	}
	err := fb.Set(value) // set, because this is a file node
	if err != nil {
		return err
	}

	resp.Size = len(req.Data)
	return nil
}
