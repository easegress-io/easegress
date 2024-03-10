/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package radixtree provides the router implementation of radix tree routing policy.
package radixtree

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// radixtree-router implementation below is a based on the original work by
// go-chi in https://github.com/go-chi/chi/blob/master/tree.go
// (MIT licensed). It's been heavily modified for use as an HTTP router.

type (
	nodeType uint8

	// Represents leaf node in radix tree
	muxPath struct {
		routers.Path
		pattern         string
		paramKeys       []string
		rewriteTemplate *template.Template
	}

	paths []*muxPath

	// Represents node and edge in radix tree
	node struct {
		// regexp matcher for regexp nodes
		rex *regexp.Regexp

		// HTTP handler endpoints on the leaf node
		paths []*muxPath

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

	muxRule struct {
		routers.Rule
		root      *node
		pathCache map[string]paths
	}

	radixTreeRouter struct {
		rules []*muxRule
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

const egWildcard = "EG_WILDCARD"

var kind = &routers.Kind{
	Name:        "RadixTree",
	Description: "RadixTree",

	CreateInstance: func(rules routers.Rules) routers.Router {
		router := &radixTreeRouter{
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
	idx, i, j := 0, 0, len(ns)-1

	for i <= j {
		idx = i + (j-i)/2
		if label > ns[idx].label {
			i = idx + 1
		} else if label < ns[idx].label {
			j = idx - 1
		} else {
			break
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
	defer func() {
		n.children[child.typ] = append(n.children[child.typ], child)
		n.children[child.typ].Sort()
	}()

	// Parse next segment
	seg := patNextSegment(prefix)

	if seg.nodeType == ntStatic {
		return child
	}

	// Route has some param
	if seg.ps > 0 {
		// starts with a static segment
		child.typ = ntStatic
		child.prefix = prefix[:seg.ps]
		child.rex = nil

		// add the param edge node
		prefix = prefix[seg.ps:]

		grandchild := &node{
			typ:    seg.nodeType,
			label:  prefix[0],
			tail:   seg.tail,
			prefix: prefix,
		}
		return child.addChild(grandchild, prefix)
	}

	// prefix prefix contains a param, regexp or wildcard
	// Route starts with a param
	child.typ = seg.nodeType

	if seg.nodeType == ntRegexp {
		rex, err := regexp.Compile(seg.rexpat)
		if err != nil {
			panic(fmt.Sprintf("invalid regexp pattern '%s' in route param", seg.rexpat))
		}
		child.prefix = seg.rexpat
		child.rex = rex
	}
	child.tail = seg.tail // for params, we set the tail

	if seg.pe != len(prefix) {
		// add static edge for the remaining part, split the end.
		// its not possible to have adjacent param nodes, so its certainly
		// going to be a static node next.

		// prefix require update?
		if seg.nodeType != ntRegexp {
			child.prefix = prefix[:seg.pe]
		}
		prefix = prefix[seg.pe:] // advance prefix position

		grandchild := &node{
			typ:    ntStatic, // after update
			label:  prefix[0],
			prefix: prefix,
		}
		return child.addChild(grandchild, prefix)
	}

	return child
}

func (n *node) replaceChild(label, tail byte, child *node) {
	ns := n.children[child.typ]
	for i := 0; i < len(ns); i++ {
		if ns[i].label == label && ns[i].tail == tail {
			ns[i] = child
			ns[i].label = label
			ns[i].tail = tail
			return
		}
	}
	panic("replacing missing child")
}

func (n *node) addPath(path *routers.Path) {
	if n.paths == nil {
		n.paths = make([]*muxPath, 0)
	}

	mp := newMuxPath(path)
	n.paths = append(n.paths, mp)
}

func (n *node) insert(path *routers.Path) {
	if path == nil {
		panic("param invalid")
	}

	search := path.Path
	parent := n

	for {
		if len(search) == 0 {
			parent.addPath(path)
			break
		}

		label := search[0]

		var seg segment
		if label == '{' || label == '*' {
			seg = patNextSegment(search)
		}

		nn := parent.getEdge(seg.nodeType, label, seg.tail, seg.rexpat)
		if nn == nil {
			child := &node{label: label, tail: seg.tail, prefix: search}
			nn = parent.addChild(child, search)
			nn.addPath(path)
			break
		}

		if nn.typ > ntStatic {
			search = search[seg.pe:]
			parent = nn
			continue
		}

		// n.prefix compare
		commonPrefix := longestPrefix(search, nn.prefix)
		if commonPrefix == len(nn.prefix) {
			search = search[commonPrefix:]
			parent = nn
			continue
		}

		// split the node
		child := &node{
			typ:    ntStatic,
			prefix: search[:commonPrefix],
		}

		parent.replaceChild(label, seg.tail, child)

		nn.label = nn.prefix[commonPrefix]
		nn.prefix = nn.prefix[commonPrefix:]
		child.addChild(nn, nn.prefix)

		search = search[commonPrefix:]
		if len(search) == 0 {
			child.addPath(path)
			break
		}

		grandchild := &node{
			typ:    ntStatic,
			label:  search[0],
			prefix: search,
		}

		nn = child.addChild(grandchild, search)
		nn.addPath(path)
		break
	}
}

func (n *node) match(context *routers.RouteContext) *muxPath {
	for _, r := range n.paths {
		if r.Match(context) {
			return r
		}
	}

	return nil
}

func (n *node) find(path string, context *routers.RouteContext) *muxPath {
	for t, nds := range n.children {
		if len(nds) == 0 {
			continue
		}

		switch ntype := nodeType(t); ntype {
		case ntStatic:
			if path == "" {
				continue
			}

			var label byte
			if path != "" {
				label = path[0]
			}
			xn := nds.findEdge(label)
			if xn == nil || !strings.HasPrefix(path, xn.prefix) {
				continue
			}

			search := path[len(xn.prefix):]
			if len(search) == 0 && xn.isLeaf() {
				if r := xn.match(context); r != nil {
					return r
				}
			}
			if fin := xn.find(search, context); fin != nil {
				return fin
			}

		case ntParam, ntRegexp:
			if path == "" {
				continue
			}

			for _, xn := range nds {
				p := strings.IndexByte(path, xn.tail)
				if p < 0 {
					if xn.tail == '/' {
						p = len(path)
					} else {
						continue
					}
				} else if ntype == ntRegexp && p == 0 {
					continue
				}

				if ntype == ntRegexp {
					if !xn.rex.MatchString(path[:p]) {
						continue
					}
				} else if strings.IndexByte(path[:p], '/') != -1 {
					continue
				}

				prevlen := len(context.Params.Values)
				context.Params.Values = append(context.Params.Values, path[:p])

				search := path[p:]
				if len(search) == 0 && xn.isLeaf() {
					if r := xn.match(context); r != nil {
						return r
					}
				}
				if fin := xn.find(search, context); fin != nil {
					return fin
				}

				context.Params.Values = context.Params.Values[:prevlen]
			}

		default:
			xn := nds[0]
			if r := xn.match(context); r != nil {
				context.Params.Values = append(context.Params.Values, path)
				return r
			}
		}
	}

	return nil
}

func (n *node) isLeaf() bool {
	return n.paths != nil
}

func newMuxPath(path *routers.Path) *muxPath {
	paramKeys := patParamKeys(path.Path)

	mp := &muxPath{
		Path:      *path,
		pattern:   path.Path,
		paramKeys: paramKeys,
	}

	mp.initRewrite()

	return mp
}

func (mp *muxPath) initRewrite() {
	if mp.RewriteTarget == "" {
		return
	}

	if strings.Contains(mp.RewriteTarget, "*") {
		panic("artRouter RewriteTarget not support '*', please use {EG_WILDCARD}")
	}

	rewriteKeys := patParamKeys(mp.RewriteTarget)

	if len(rewriteKeys) == 0 {
		// nStatic rewriteTarget
		return
	}

	repl := mp.RewriteTarget

	for _, rk := range rewriteKeys {
		if !stringtool.StrInSlice(rk, mp.paramKeys) {
			panic("artRouter RewriteTarget syntax error: " + mp.RewriteTarget)
		}
		newRk := "{{ ." + rk + "}}"
		repl = strings.ReplaceAll(repl, "{"+rk+"}", newRk)
	}

	mp.rewriteTemplate = template.Must(template.New("").Parse(repl))
}

func (mp *muxPath) Protocol() string {
	return "http"
}

func (mp *muxPath) Rewrite(context *routers.RouteContext) {
	req := context.Request

	if mp.RewriteTarget == "" {
		return
	}

	if mp.rewriteTemplate == nil {
		req.SetPath(mp.RewriteTarget)
		return
	}

	var buf bytes.Buffer
	mp.rewriteTemplate.Execute(&buf, context.GetCaptures())
	req.SetPath(buf.String())
}

func newMuxRule(rule *routers.Rule) *muxRule {
	mr := &muxRule{
		Rule:      *rule,
		root:      &node{},
		pathCache: make(map[string]paths),
	}

	for _, path := range rule.Paths {
		seg := patNextSegment(path.Path)

		if seg.nodeType == ntStatic {
			p := path.Path
			mr.pathCache[p] = append(mr.pathCache[p], newMuxPath(path))
		} else {
			mr.root.insert(path)
		}
	}

	return mr
}

func (r *radixTreeRouter) Search(context *routers.RouteContext) {
	path := context.Path
	req := context.Request
	ip := req.RealIP()

	for _, rule := range r.rules {
		if !rule.MatchHost(context) {
			continue
		}

		if !rule.AllowIP(ip) {
			context.IPMismatch = true
			continue
		}

		if paths, ok := rule.pathCache[path]; ok {
			for _, mp := range paths {
				if mp.Match(context) {
					context.Route = mp
					return
				}
			}
		}

		mp := rule.root.find(path, context)

		if mp != nil {
			context.Route = mp
			context.Params.Keys = append(context.Params.Keys, mp.paramKeys...)
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

	// Wildcard pattern as final
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
		key:      egWildcard,
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
