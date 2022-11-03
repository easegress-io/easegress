/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package art

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/megaease/easegress/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

// art-router implementation below is a based on the original work by
// go-chi in https://github.com/go-chi/chi/blob/master/tree.go
// (MIT licensed). It's been heavily modified for use as a HTTP router.

type (
	nodeType uint8

	// Represents leaf node in radix tree
	route struct {
		routers.Path
		pattern         string
		paramKeys       []string
		rewriteTemplate *template.Template
	}

	routes []*route

	// Represents node and edge in radix tree
	node struct {
		// regexp matcher for regexp nodes
		rex *regexp.Regexp

		// HTTP handler endpoints on the leaf node
		routes []*route

		// prefix is the common prefix we ignore
		prefix string

		// child nodes should be stored in-order for iteration,
		// in groups of the node type.
		children [ntCatchAll + 1]nodes

		// first byte of the child prefix
		tail byte

		// node type: static, regexp, param, catchAll
		typ nodeType

		// first byte of the prefix
		label byte
	}

	nodes []*node

	PathCache map[string]routes

	muxRule struct {
		routers.Rule
		root      *node
		pathCache PathCache
	}

	ArtRouter struct {
		rules []*muxRule
	}

	routeParams struct {
		Keys, Values []string
	}

	segment struct {
		key      string
		rexpat   string
		ps       int
		pe       int
		nodeType nodeType
		tail     byte
	}
)

const (
	ntStatic   nodeType = iota // /home
	ntRegexp                   // /{id:[0-9]+}
	ntParam                    // /{user}
	ntCatchAll                 // /api/v1/*
)

const EG_WILDCARD = "EG_WILDCARD"

// Kind is the kind of Proxy.
const Kind = "Art"

var kind = &routers.Kind{
	Name:        Kind,
	Description: "Art",

	CreateInstance: func(rules routers.Rules) routers.Router {
		router := &ArtRouter{
			rules: make([]*muxRule, len(rules)),
		}

		for i, rule := range rules {
			mr := newMuxRule(rule)
			router.rules[i] = mr
		}

		return router
	},
}

func init() {
	routers.Register(kind)
}

// Sort the list of nodes by label
func (ns nodes) Sort()              { sort.Sort(ns); ns.tailSort() }
func (ns nodes) Len() int           { return len(ns) }
func (ns nodes) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns nodes) Less(i, j int) bool { return ns[i].label < ns[j].label }

// tailSort pushes nodes with '/' as the tail to the end of the list for param nodes.
// The list order determines the traversal order.
func (ns nodes) tailSort() {
	for i := len(ns) - 1; i >= 0; i-- {
		// param node label is {,
		if ns[i].typ > ntStatic && ns[i].tail == '/' {
			ns.Swap(i, len(ns)-1)
			return
		}
	}
}

func (ns nodes) findEdge(label byte) *node {
	// static nodes find
	num := len(ns)
	idx := 0
	i, j := 0, num-1
	for i <= j {
		idx = i + (j-i)/2
		if label > ns[idx].label {
			i = idx + 1
		} else if label < ns[idx].label {
			j = idx - 1
		} else {
			i = num // breaks cond
		}
	}
	if ns[idx].label != label {
		return nil
	}
	return ns[idx]
}

func (n *node) getEdge(ntyp nodeType, label, tail byte, rexpat string) *node {
	childs := n.children[ntyp]
	for i := 0; i < len(childs); i++ {
		if childs[i].label == label && childs[i].tail == tail {
			if ntyp == ntRegexp && childs[i].prefix != rexpat {
				continue
			}
			return childs[i]
		}
	}

	return nil
}

// addChild appends the new `child` node to the tree using the `pattern` as the trie key.
// For a URL router like chi's, we split the static, param, regexp and wildcard segments
// into different nodes. In addition, addChild will recursively call itself until every
// pattern segment is added to the url pattern tree as individual nodes, depending on type.
func (n *node) addChild(child *node, prefix string) *node {
	search := prefix

	// handler leaf node added to the tree is the child.
	// this may be overridden later down the flow
	hn := child

	// Parse next segment
	seg := patNextSegment(search)

	segType := seg.nodeType

	// Add child depending on next up segment
	switch segType {

	case ntStatic:
		// Search prefix is all static (that is, has no params in path)
		// noop

	default:
		// Search prefix contains a param, regexp or wildcard

		ps := seg.ps
		pe := seg.pe

		if ps == 0 {
			// Route starts with a param
			child.typ = segType

			if segType == ntRegexp {
				rex, err := regexp.Compile(seg.rexpat)
				if err != nil {
					panic(fmt.Sprintf("invalid regexp pattern '%s' in route param", seg.rexpat))
				}
				child.prefix = seg.rexpat
				child.rex = rex
			}

			child.tail = seg.tail // for params, we set the tail

			if pe != len(search) {
				// add static edge for the remaining part, split the end.
				// its not possible to have adjacent param nodes, so its certainly
				// going to be a static node next.

				// prefix require update?
				if segType != ntRegexp {
					child.prefix = search[:pe]
				}

				search = search[pe:] // advance search position

				nn := &node{
					typ:    ntStatic, // after update
					label:  search[0],
					prefix: search,
				}
				hn = child.addChild(nn, search)
			}

		} else if ps > 0 {
			// Route has some param

			// starts with a static segment
			child.typ = ntStatic
			child.prefix = search[:ps]
			child.rex = nil

			// add the param edge node
			search = search[ps:]

			nn := &node{
				typ:    segType,
				label:  search[0],
				tail:   seg.tail,
				prefix: search,
			}
			hn = child.addChild(nn, search)
		}
	}

	n.children[child.typ] = append(n.children[child.typ], child)
	n.children[child.typ].Sort()
	return hn
}

func (n *node) replaceChild(label, tail byte, child *node) {
	for i := 0; i < len(n.children[child.typ]); i++ {
		if n.children[child.typ][i].label == label && n.children[child.typ][i].tail == tail {
			n.children[child.typ][i] = child
			n.children[child.typ][i].label = label
			n.children[child.typ][i].tail = tail
			return
		}
	}
	panic("replacing missing child")
}

func (n *node) setRoute(path *routers.Path) {
	if n.routes == nil {
		n.routes = make([]*route, 0)
	}

	r := newRoute(path)
	n.routes = append(n.routes, r)
}

func (root *node) insert(path *routers.Path) (*node, error) {
	if path == nil {
		panic("param invalid")
	}

	search := path.Path

	var parent *node
	n := root

	for {

		if len(search) == 0 {
			n.setRoute(path)
			return n, nil
		}

		label := search[0]

		var seg segment
		if label == '{' || label == '*' {
			seg = patNextSegment(search)
		}

		parent = n
		n = n.getEdge(seg.nodeType, label, seg.tail, seg.rexpat)

		if n == nil {
			child := &node{label: label, tail: seg.tail, prefix: search}
			hn := parent.addChild(child, search)
			hn.setRoute(path)
			return hn, nil
		}

		if n.typ > ntStatic {
			search = search[seg.pe:]
			continue
		}

		// n.prefix compare
		commonPrefix := longestPrefix(search, n.prefix)

		if commonPrefix == len(n.prefix) {
			search = search[commonPrefix:]
			continue
		}

		// split the node
		child := &node{
			typ:    ntStatic,
			prefix: search[:commonPrefix],
		}

		parent.replaceChild(label, seg.tail, child)

		n.label = n.prefix[commonPrefix]
		n.prefix = n.prefix[commonPrefix:]
		child.addChild(n, n.prefix)

		search = search[commonPrefix:]
		if len(search) == 0 {
			child.setRoute(path)
			return child, nil
		}

		subChild := &node{
			typ:    ntStatic,
			label:  search[0],
			prefix: search,
		}

		hn := child.addChild(subChild, search)
		hn.setRoute(path)
		return hn, nil

	}
}

func (n *node) match(context *routers.RouteContext) *route {
	for _, r := range n.routes {
		if r.Match(context) {
			return r
		}
	}

	return nil
}

func (n *node) find(path string, context *routers.RouteContext) *route {
	nn := n
	search := path

	for t, nds := range nn.children {

		if len(nds) == 0 {
			continue
		}

		ntype := nodeType(t)

		var xn *node
		xsearch := search

		var label byte

		if search != "" {
			label = search[0]
		}

		switch ntype {
		case ntStatic:

			if xsearch == "" {
				continue
			}

			xn = nds.findEdge(label)
			if xn == nil || !strings.HasPrefix(xsearch, xn.prefix) {
				continue
			}
			xsearch = xsearch[len(xn.prefix):]

			if len(xsearch) == 0 {
				if xn.isLeaf() {
					r := xn.match(context)
					if r != nil {
						return r
					}
				}
			}
			fin := xn.find(xsearch, context)
			if fin != nil {
				return fin
			}

		case ntParam, ntRegexp:
			if xsearch == "" {
				continue
			}

			for idx := 0; idx < len(nds); idx++ {
				xn = nds[idx]

				p := strings.IndexByte(xsearch, xn.tail)

				if p < 0 {
					if xn.tail == '/' {
						// xsearch is param value
						p = len(search)
					} else {
						continue
					}
				} else if ntype == ntRegexp && p == 0 {
					continue
				}

				if ntype == ntRegexp {
					if !xn.rex.MatchString(xsearch[:p]) {
						continue
					}
				} else if strings.IndexByte(xsearch[:p], '/') != -1 {
					continue
				}

				prevlen := len(context.RouteParams.Values)
				context.RouteParams.Values = append(context.RouteParams.Values, xsearch[:p])
				xsearch = xsearch[p:]

				if len(xsearch) == 0 {
					if xn.isLeaf() {
						r := xn.match(context)
						if r != nil {
							return r
						}
					}
				}
				fin := xn.find(xsearch, context)
				if fin != nil {
					return fin
				}
				context.RouteParams.Values = context.RouteParams.Values[:prevlen]
				xsearch = search
			}

		default:
			xn = nds[0]
			r := xn.match(context)
			if r != nil {
				context.RouteParams.Values = append(context.RouteParams.Values, xsearch)
				return r
			}
		}

	}

	return nil
}

func (n *node) isLeaf() bool {
	return n.routes != nil
}

func newRoute(path *routers.Path) *route {
	paramKeys := patParamKeys(path.Path)

	r := &route{
		Path:      *path,
		pattern:   path.Path,
		paramKeys: paramKeys,
	}

	r.initRewrite()

	return r
}

func (r *route) initRewrite() {
	if r.RewriteTarget == "" {
		return
	}

	repl := r.RewriteTarget
	t := template.Must(template.New("").Parse(repl))
	root := t.Root
	var params []string
	for _, n := range root.Nodes {
		t := n.Type()
		if t != parse.NodeText && t != parse.NodeAction {
			panic("artRouter RewriteTarget syntax error: " + r.RewriteTarget)
		}
		if t == parse.NodeAction {
			pos := int(n.Position()) + 1
			end := pos + strings.Index(repl[pos:], "}}")
			param := strings.TrimSpace(repl[pos:end])
			if !stringtool.StrInSlice(param, r.paramKeys) {
				panic("artRouter RewriteTarget syntax error: " + r.RewriteTarget)
			}
			params = append(params, param)
		}
	}

	if len(params) != 0 {
		r.rewriteTemplate = t
	}
}

func (r *route) Rewrite(context *routers.RouteContext) {
	req := context.Request

	if r.RewriteTarget == "" {
		return
	}

	if r.rewriteTemplate == nil {
		req.SetPath(r.RewriteTarget)
		return
	}

	var buf bytes.Buffer
	r.rewriteTemplate.Execute(&buf, context.GetCaptures())
	req.SetPath(buf.String())
}

func (pc PathCache) addPath(path *routers.Path) {
	p := path.Path
	if _, ok := pc[p]; ok {
		pc[p] = append(pc[p], newRoute(path))
	} else {
		pc[p] = []*route{newRoute(path)}
	}
}

func newMuxRule(rule *routers.Rule) *muxRule {
	mr := &muxRule{
		Rule:      *rule,
		root:      &node{},
		pathCache: make(PathCache),
	}

	for _, path := range rule.Paths {

		seg := patNextSegment(path.Path)

		if seg.nodeType == ntStatic {
			mr.pathCache.addPath(path)
		} else {
			_, err := mr.root.insert(path)
			if err != nil {
				panic(err)
			}
		}
	}

	return mr
}

func (ar *ArtRouter) Search(context *routers.RouteContext) {
	path := context.Path
	req := context.Request
	ip := req.RealIP()

	for _, rule := range ar.rules {
		if !rule.Match(req) {
			continue
		}

		if !rule.AllowIP(ip) {
			context.IPNotAllowed = true
			return
		}

		if routes, ok := rule.pathCache[path]; ok {
			for _, route := range routes {
				if route.Match(context) {
					context.Route = route
					return
				}
			}
		}

		route := rule.root.find(path, context)

		if route != nil {
			context.Route = route
			context.RouteParams.Keys = append(context.RouteParams.Keys, route.paramKeys...)
			return
		}
	}
}

func patNextSegment(pattern string) segment {
	ps := strings.Index(pattern, "{")
	ws := strings.Index(pattern, "*")

	if ps < 0 && ws < 0 {
		// we return the entire thing
		return segment{
			nodeType: ntStatic,
			pe:       len(pattern),
		}
	}

	// Sanity check
	if ps >= 0 && ws >= 0 && ws < ps {
		panic("wildcard '*' must be the last pattern in a route, otherwise use a '{param}'")
	}

	// Wildcard pattern as finale

	if ps >= 0 {

		var tail byte = '/' // Default endpoint tail to / byte
		// Param/Regexp pattern is next
		nt := ntParam

		// Read to closing } taking into account opens and closes in curl count (cc)
		cc := 0
		pe := ps
		for i, c := range pattern[ps:] {
			if c == '{' {
				cc++
			} else if c == '}' {
				cc--
				if cc == 0 {
					pe = ps + i
					break
				}
			}
		}
		if pe == ps {
			panic("route param closing delimiter '}' is missing")
		}

		key := pattern[ps+1 : pe]
		pe++ // set end to next position

		if pe < len(pattern) {
			tail = pattern[pe]
		}

		var rexpat string
		if idx := strings.Index(key, ":"); idx >= 0 {
			nt = ntRegexp
			rexpat = key[idx+1:]
			key = key[:idx]
		}

		if len(rexpat) > 0 {
			if rexpat[0] != '^' {
				rexpat = "^" + rexpat
			}
			if rexpat[len(rexpat)-1] != '$' {
				rexpat += "$"
			}
		}

		return segment{
			nodeType: nt,
			key:      key,
			rexpat:   rexpat,
			tail:     tail,
			ps:       ps,
			pe:       pe,
		}
	}

	if ws < len(pattern)-1 {
		panic("wildcard '*' must be the last value in a route. trim trailing text or use a '{param}' instead")
	}

	return segment{
		nodeType: ntCatchAll,
		key:      EG_WILDCARD,
		ps:       ws,
		pe:       len(pattern),
	}
}

func patParamKeys(pattern string) []string {
	pat := pattern
	paramKeys := []string{}
	for {
		seg := patNextSegment(pat)
		if seg.nodeType == ntStatic {
			return paramKeys
		}
		for i := 0; i < len(paramKeys); i++ {
			if paramKeys[i] == seg.key {
				panic(fmt.Sprintf("routing pattern '%s' contains duplicate param key, '%s'", pattern, seg.key))
			}
		}
		paramKeys = append(paramKeys, seg.key)
		pat = pat[seg.pe:]
	}
}

// longestPrefix finds the length of the shared prefix
// of two strings
func longestPrefix(k1, k2 string) int {
	max := len(k1)
	if l := len(k2); l < max {
		max = l
	}
	var i int
	for i = 0; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}
