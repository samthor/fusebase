package local

import (
	"fmt"
	"strings"
	"time"
)

// Node represents a node in Firebase.
type Node struct {
	Created time.Time
	Updated time.Time
	Data    interface{}
}

// NodeMap is used as Data within Node when it is a map.
type NodeMap map[string]*Node

// Print renders data rooted at this Node.
func (n *Node) Print() {
	n.internalPrint("")
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
		child = &Node{Created: now}
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

		for k, sub := range m {
			node := &Node{Created: now}
			if !node.set(now, sub) {
				local[k] = node
			}
		}
		return len(local) == 0 // nuke if we didn't end up with any keys for some reason
	}

	// nb. should be a bool, float64 or string
	n.Data = data
	return false
}
