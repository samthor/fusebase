package fusebase

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/samthor/fusebase/local"

	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/net/context"
)

const (
	debug = false
)

// TODO: None of this is threadsafe. Updates come in on their own goroutine,
// and it's unclear where bazil.org/fuse library calls these handlers from.

func (f *FUSEBase) Root() (fs.Node, error) {
	return f.fs, nil
}

func (f *FUSEBase) wrapLocalNode(ln *local.Node) fs.Node {
	out := f.nodes[ln]
	if out == nil {
		out = &fsNode{Node: ln, f: f}
		f.nodes[ln] = out

		defer func() {
			go func() {
				out.users.Wait()
				delete(f.nodes, ln)
				log.Printf("local node closed: %p (%v)", out, ln.Key)
			}()
		}()
	}
	out.users.Add(1)
	log.Printf("loaded node: %p (%v)", out, ln.Key)

	wrap := wrapNode(out, func() {
		out.users.Done()
	})
	return wrap
}

type fsNode struct {
	Node  *local.Node
	f     *FUSEBase
	b     []byte // cache
	users sync.WaitGroup
}

func (node *fsNode) isDir() bool {
	return node.Node.Map() != nil
}

func (node *fsNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = node.Node.Inode
	if node.isDir() {
		a.Mode = os.ModeDir | 0755
	} else {
		node.b = node.Node.Bytes()
		a.Size = uint64(len(node.b))
		a.Mode = 0644
	}
	if node.Node.VNew {
		a.Mode ^= 0444 // pretend you can't read it
		a.Mode |= os.ModeTemporary
	}
	return nil
}

func (node *fsNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	sub, _ := node.Node.Lookup(name)
	if sub == nil || !sub.Show() {
		// does not exist or not a dir
		return nil, fuse.ENOENT
	}
	return node.f.wrapLocalNode(sub), nil
}

func (node *fsNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if !local.ValidKey(req.Name) {
		return nil, nil, EINVAL
	}
	sub, err := node.Node.Create(time.Now(), req.Name)
	if err != nil {
		return nil, nil, err
	}
	out := node.f.wrapLocalNode(sub)
	return out, out, nil
}

func (node *fsNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	if !local.ValidKey(req.Name) {
		return nil, EINVAL
	}
	sub, err := node.Node.Create(time.Now(), req.Name)
	if err != nil {
		return nil, err
	}
	sub.Data = make(local.NodeMap)
	return node.f.wrapLocalNode(sub), nil
}

func (node *fsNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	m := node.Node.Map()
	sub := m[req.Name]
	if sub != nil && sub.VNew && !sub.VMod {
		delete(m, req.Name) // local deletion is fine
		return nil
	}

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

	out := make([]fuse.Dirent, 0, len(m)+2)
	out = append(out, fuse.Dirent{Name: "."}, fuse.Dirent{Name: ".."})

	for k, v := range m {
		ent := fuse.Dirent{Inode: v.Inode, Name: k}
		if v.Data == nil && !v.VNew {
			continue // no data, perhaps pending
		} else if v.Map() != nil {
			ent.Type = fuse.DT_Dir
		} else {
			ent.Type = fuse.DT_File
		}
		out = append(out, ent)

		if !debug {
			continue
		}
		if v.VNew {
			ent = fuse.Dirent{Inode: 1, Name: k + "$VNew"}
			out = append(out, ent)
		}
		if v.VMod {
			ent = fuse.Dirent{Inode: 1, Name: k + "$VMod"}
			out = append(out, ent)
		}
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
		// TODO: this is valid on new files?
		//log.Printf("set with nil existing data: %v", node.Node.Key)
		//return fuse.EIO
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
