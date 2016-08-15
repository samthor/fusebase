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

	value := bytesToValue(req.Data)

	// TODO: This is a synchronous write that doesn't update the local state. We probably want to
	// optimistically set locally and send the request later (support offline). But Firebase has no
	// versioning so we have no idea when our change comes back.
	log.Printf("got set for: %v => %v\n", node.Node.Key, value)
	fb := node.f.f
	if node.Node.Key != "" {
		fb = fb.Child(node.Node.Key)
	}
	err := fb.Set(value) // set, because this is a file node
	if err != nil {
		log.Printf("couldn't write, got err: %v", err)
		return err
	}
	log.Printf("write ok");

	resp.Size = len(req.Data)
	return nil
}

func bytesToValue(b []byte) interface{} {
	// TODO: js true/false?
	s := string(b)
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64) // need to TrimSpace for newline
	if err == nil {
		return f
	}
	return s // nb. echo generates newlines, most people don't want them
}
