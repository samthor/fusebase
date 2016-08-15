package fusebase

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/samthor/fusebase/local"

	"log"
	"os"
	"strconv"
	"strings"

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

func (node *fsNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = node.Node.Inode
	if node.Node.Map() != nil {
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

func (node *fsNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// TODO: explicitly prevent directories
	// allow write at zero, _or_ at end (replaces value)
	if req.Offset != 0 && int(req.Offset) < len(node.b) {
		return fuse.EIO
	}
	if node.Node.Data == nil {
		// TODO: new files currently unsupported
		return fuse.EIO
	}

	value := bytesToValue(req.Data, node.Node.Data)

	// TODO: This is a synchronous write that doesn't update the local state. We probably want to
	// optimistically set locally and send the request later (support offline), then wait until the
	// update comes in (during the write, or immediately after it).
	log.Printf("set: %v => %v\n", node.Node.Key, value)
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

// bytesToValue converts the written bytes to a native type to write to Firebase, incorporating
// the node's previous value (if any).
func bytesToValue(b []byte, prev interface{}) interface{} {
	if _, ok := prev.(string); ok {
		return string(b) // previously a string, assume string
	}

	s := string(b)
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64) // need to TrimSpace for newline
	if err != nil {
		return s // string (nb. echo generates newlines, most people don't want them)
	}
	if _, ok := prev.(bool); ok {
		return f != 0 // prev was bool, assume number means bool again
	}
	return f // number
}
