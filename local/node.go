package local

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// Node represents a node in Firebase.
type Node struct {
	Created time.Time
	Updated time.Time
	Inode   uint64 // guaranteed unique inode
	Key     string

	Data interface{}

	VNew bool // virtual new (local create)
	VMod bool // virtual modification (local write on existing file)
}

// NodeMap is used as Data within Node when it is a map.
type NodeMap map[string]*Node

// Bytes returns this Node as bytes, for primitive nodes.
func (n *Node) Bytes() []byte {
	switch d := n.Data.(type) {
	case int64:
		return strconv.AppendInt(nil, d, 10)
	case float64:
		return strconv.AppendFloat(nil, d, 'f', -1, 64)
	case bool:
		// TODO: use a special node type to indicate bool
		if d {
			return []byte("1")
		}
		return []byte("0")
	case string:
		return []byte(d)
	}
	return nil
}

// Show determines whether this node should be shown.
func (n *Node) Show() bool {
	return n.Data != nil || n.VNew
}

// Print renders data rooted at this Node.
func (n *Node) Print() {
	n.internalPrint("")
}

// Map returns the NodeMap for this Node, or nil if it is not an object/dir.
func (n *Node) Map() NodeMap {
	if m, ok := n.Data.(NodeMap); ok {
		return m
	}
	return nil
}

// Create creates a local-only node under the given key.
func (n *Node) Create(now time.Time, key string) (*Node, error) {
	if !ValidKey(key) {
		return nil, fmt.Errorf("invalid key: %v", key)
	}
	m, ok := n.Data.(NodeMap)
	if !ok {
		return nil, fmt.Errorf("can only create in dir, was: %v", n.Data)
	} else if prev, ok := m[key]; ok {
		return prev, nil // already exists, probably fine
	}

	child := &Node{
		Created: now,
		Inode:   <-inodeCh,
		Key:     n.Key + "/" + key,
		VNew:    true,
	}
	m[key] = child
	return child, nil
}

// Lookup returns the node under this key, if any.
func (n *Node) Lookup(key string) (*Node, bool) {
	if m, ok := n.Data.(NodeMap); ok {
		return m[key], true
	}
	return nil, false
}

// Handle sets the given path to the specified raw data.
func (n *Node) Handle(now time.Time, path string, data interface{}) error {
	if path == "" || path[0] != '/' {
		return fmt.Errorf("invalid path: %v", path)
	}
	p := []string{}
	if path != "/" {
		p = strings.Split(path[1:], "/")
	}
	n.internalHandle(now, p, data)
	return nil
}

// purgeNode purges this Node and all its descendants.
func purgeNode(n *Node) {
	pending := []*Node{n}

	for i := 0; i < len(pending); i++ {
		next := pending[i]
		if len(next.Key) == 0 || next.Key[0] != '/' {
			log.Printf("can't purge node, unexpected key: %v", next.Key)
		} else {
			// TODO: anything not starting with "/" is dead
			next.Key = "-" + next.Key
		}

		if m, ok := n.Data.(NodeMap); ok {
			for _, node := range m {
				pending = append(pending, node)
			}
		}
	}
}

func (n *Node) remoteTouch(now time.Time) {
	n.Updated = now
	n.VNew = false
	n.VMod = false
}

func (n *Node) internalHandle(now time.Time, p []string, data interface{}) bool {
	if len(p) == 0 {
		// TODO: directories can become files and vice versa
		return n.set(now, data)
	}

	m, ok := n.Data.(NodeMap)
	if !ok {
		// TODO: We're not purging here: it's the _same_ node, but it is a transition primitive => dict.
		m = make(NodeMap) // weird, but Firebase thinks there's a map here
		n.Data = m
		// TODO: crash?
	}

	key := p[0]
	child := m[key]
	if child == nil {
		if data == nil {
			return false // don't bother proceeding, nuking anyway
		}
		child = &Node{Created: now, Inode: <-inodeCh, Key: n.Key + "/" + key}
		m[key] = child
	} else {
		child.remoteTouch(now)
	}
	nuked := child.internalHandle(now, p[1:], data)
	if nuked {
		delete(m, key)
		purgeNode(child)
	}

	// does this node have any real, i.e. non-VNew nodes?
	hasReal := false
	for _, sub := range m {
		if !sub.VNew {
			hasReal = true
			break
		}
	}
	if !hasReal {
		n.VNew = true // if not, then it's basically a VNew node
	}

	return len(m) == 0 // nuke if no more children here
}

func (n *Node) internalPrint(prefix string) {
	if m, ok := n.Data.(NodeMap); ok {
		fmt.Print("{\n")
		updatePrefix := "  " + prefix
		for k, sub := range m {
			fmt.Printf("%s%s: ", updatePrefix, k)
			sub.internalPrint(updatePrefix)
		}
		fmt.Printf("%s}\n", prefix)
	} else {
		fmt.Printf("%v\n", n.Data)
	}
}

// set updates the given Node with new data. Returns true if the Node should be nuked by its owner.
func (n *Node) set(now time.Time, data interface{}) bool {
	n.remoteTouch(now)
	if data == nil {
		n.Data = nil // incase we're the root node or someone is holding onto us
		return true
	}

	if m, ok := data.(map[string]interface{}); ok {
		local := make(NodeMap)
		n.Data = local

		for key, sub := range m {
			node := &Node{Created: now, Inode: <-inodeCh, Key: n.Key + "/" + key}
			if !node.set(now, sub) {
				local[key] = node
			}
		}
		return len(local) == 0 // nuke if we didn't end up with any keys for some reason
	}

	// nb. should be a primitive type
	n.Data = data
	return false
}
