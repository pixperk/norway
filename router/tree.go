package router

import (
	"net/http"
	"strings"
)

// the radix has three types of nodes
// STATIC : api/v1/users
// PARAM : api/v1/:id -> capture id as a parameter
// WILDCARD : api/v1/*filepath -> capture the rest of the path as filepath parameter
type node struct {
	path       string
	children   []*node
	handler    http.Handler
	isParam    bool
	isWildcard bool
	paramName  string
}

func (n *node) insertSegment(seg string) *node {
	//param or wildcard segment
	switch seg[0] {
	case ':':
		//first check if a param child already exists, if so, return it
		for _, child := range n.children {
			if child.isParam {
				return child
			}
		}

		//otherwise create a new param child
		child := &node{path: seg, isParam: true, paramName: seg[1:]}
		n.children = append(n.children, child)
		return child
	case '*':
		//first check if a wildcard child already exists, if so, return it
		for _, child := range n.children {
			if child.isWildcard {
				return child
			}
		}

		//otherwise create a new wildcard child
		child := &node{path: seg, isWildcard: true, paramName: seg[1:]}
		n.children = append(n.children, child)
		return child
	}

	//static segment : radix logic
	for _, child := range n.children {
		if child.isParam || child.isWildcard {
			continue
		}
		common := longestPrefix(seg, child.path)
		if common == 0 {
			continue
		}

		//exact match, descend into child
		if common == len(child.path) && common == len(seg) {
			return child
		}

		//child's path is a prefix of the segment, descend into child with remaining segment
		if common == len(child.path) {
			return child.insertSegment(seg[common:])
		}

		//partial match, split the child node
		splitChild := &node{
			path:     child.path[:common],
			children: []*node{child},
			handler:  child.handler,
		}
		child.path = child.path[common:]
		child.children = []*node{splitChild}
		child.handler = nil

		if common == len(seg) {
			return child
		}

		//remainder is a new child of the split node
		newChild := &node{path: seg[common:]}
		child.children = append(child.children, newChild)
		return newChild

	}

	//no common prefix with any child, add as new child
	child := &node{path: seg}
	n.children = append(n.children, child)
	return child

}

// returns the handler for the given path, or nil if not found
func (n *node) lookup(path string, params map[string]string) http.Handler {
	for _, child := range n.children {
		//static child, does path start with child's path?
		if !child.isParam && !child.isWildcard {
			if strings.HasPrefix(path, child.path) {
				remaining := path[len(child.path):]
				if remaining == "" && child.handler != nil {
					return child.handler
				}
				if h := child.lookup(remaining, params); h != nil {
					return h
				}
			}
			continue
		}

		//param child, consume until next '/' and descend
		if child.isParam {
			slash := strings.IndexByte(path, '/')
			var segment, remaining string
			if slash == -1 {
				segment, remaining = path, ""
			} else {
				segment, remaining = path[:slash], path[slash:]
			}
			if segment == "" {
				continue
			}
			params[child.paramName] = segment
			if remaining == "" && child.handler != nil {
				return child.handler
			}
			if h := child.lookup(remaining, params); h != nil {
				return h
			}
			delete(params, child.paramName) //backtrack
			continue
		}

		//wildcard child, capture the rest of the path and return handler
		if child.isWildcard {
			params[child.paramName] = path
			return child.handler
		}
	}

	return nil
}

type Tree struct {
	root *node
}

func NewTree() *Tree {
	return &Tree{
		root: &node{},
	}
}

// insert a path to tree with its handler
func (t *Tree) Insert(path string, handler http.Handler) {
	segments := splitPath(path)
	current := t.root
	for _, seg := range segments {
		current = current.insertSegment(seg)
	}
	current.handler = handler
}

// lookup a path and return its handler and any captured parameters
func (t *Tree) Lookup(path string) (http.Handler, map[string]string) {
	params := make(map[string]string)
	handler := t.root.lookup(path, params)
	return handler, params
}

// returns the index where the path diverges from the node's path
func longestPrefix(a, b string) int {
	max := min(len(b), len(a))

	for i := 0; i < max; i++ {
		if a[i] != b[i] {
			return i
		}
	}

	return max
}

func splitPath(path string) []string {
	var segments []string
	i := 0
	for i < len(path) {
		if path[i] == ':' {
			//path segment : consume until next '/' or end of string
			j := i + 1
			for j < len(path) && path[j] != '/' {
				j++
			}
			segments = append(segments, path[i:j])
			i = j
		} else if path[i] == '*' {
			// wildcard segment: consume until end of string
			segments = append(segments, path[i:])
			break
		} else {
			// static segment: consume until next ':' or '*' or end of string
			j := i
			for j < len(path) && path[j] != ':' && path[j] != '*' {
				j++
			}
			segments = append(segments, path[i:j])
			i = j
		}
	}
	return segments
}
