package local

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	inodeCh = make(chan uint64)
)

func init() {
	globalInode := uint64(0)
	go func() {
		for {
			globalInode++
			inodeCh <- globalInode
		}
	}()
}

// Node represents a node in Firebase.
type Node struct {
	Created time.Time
	Updated time.Time
	Inode   uint64 // guaranteed unique inode
	Key     string

	Data interface{}
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
		// TODO: what's better than 1/0?
		if d {
			return []byte("1")
		}
		return []byte("0")
	case string:
		return []byte(d)
	}
	return nil
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

func (n *Node) internalHandle(now time.Time, p []string, data interface{}) bool {
	if len(p) == 0 {
		// TODO: directories can become files and vice versa
		return n.set(now, data)
	}

	m, ok := n.Data.(NodeMap)
	if !ok {
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
	}
	nuked := child.internalHandle(now, p[1:], data)
	if nuked {
		delete(m, key)
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

func (n *Node) set(now time.Time, data interface{}) bool {
	n.Updated = now
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

	// nb. should be a bool, float64 or string
	n.Data = data
	return false
}
