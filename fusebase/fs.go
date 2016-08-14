package fusebase

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/samthor/fusebase/local"

	"os"

	"golang.org/x/net/context"
)

func (f *FUSEBase) Root() (fs.Node, error) {
	return &fsNode{&f.root}, nil
}

type fsNode struct {
	Node *local.Node
}

func (node *fsNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = node.Node.Inode
	if node.Node.Map() != nil {
		a.Mode = os.ModeDir | 0555
	} else {
		a.Mode = 0644
		a.Size = 0 // uint64(len(greeting)) // TODO
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
	return &fsNode{sub}, nil
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
	return []byte{}, nil
}
