package plugins

import (
	"testing"
)

func TestParsePathNormally(t *testing.T) {
	path := "/r/megaease/easegateway/tags/"
	pattern := "/r/{user}/{repo}/tags/"

	match, ret, err := parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !match || ret == nil || len(ret) != 2 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease/easegateway/tags/server-0.1"
	pattern = "/r/{user}/{repo}/tags/"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if match {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	path = "/r/megaease/easegateway/tags/server-0.1"
	pattern = "/r/{user}/{repo}/{tag}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if match {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	path = "/r/megaease/easegateway/tags/server-0.1"
	pattern = "/r/{user}/{repo}/tags/{tag}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !match || ret == nil || len(ret) != 3 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" || ret["tag"] != "server-0.1" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease/easegateway/tags/server-0.1/foo"
	pattern = "/r/{user}/{repo}/tags/{tag}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if match {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	path = "/r/megaease/easegateway/tags/server-0.1?foo=bar"
	pattern = "/r/{user}/{repo}/tags/{tag}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if match || ret == nil || len(ret) != 3 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" || ret["tag"] != "server-0.1" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease/easegateway/tags/server-0.1?foo=bar"
	pattern = "/r/{user}/{repo}/tags/{tag}?{query}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !match || ret == nil || len(ret) != 4 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" ||
		ret["tag"] != "server-0.1" || ret["query"] != "foo=bar" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease/easegateway/tags/server-0.1/"
	pattern = "/r/{user}/{repo}/tags/{tag}/{none}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if match || ret == nil || len(ret) != 3 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" || ret["tag"] != "server-0.1" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease/easegateway/tags/server-0.1/foo"
	pattern = "/r/{user}/{repo}/tags/{tag}/foo/{none}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if match || ret == nil || len(ret) != 3 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" || ret["tag"] != "server-0.1" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease/easegateway/tags/server-0.1/foo/bar"
	pattern = "/r/{user}/{repo}/tags/{tag}/foo/{bar}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !match || ret == nil || len(ret) != 4 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["user"] != "megaease" || ret["repo"] != "easegateway" ||
		ret["tag"] != "server-0.1" || ret["bar"] != "bar" {
		t.Fatalf("unexpected parse result %v", ret)
	}

	path = "/r/megaease"
	pattern = "/r/megaease"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !match || ret == nil || len(ret) != 0 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	path = "/{foo}/bar"
	pattern = "/{foo}/{bar}"

	match, ret, err = parsePath(path, pattern)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !match || ret == nil || len(ret) != 2 {
		t.Fatalf("unexpected parse return %v, %v, %v", match, ret, len(ret))
	}

	if ret["foo"] != "{foo}" || ret["bar"] != "bar" {
		t.Fatalf("unexpected parse result %v", ret)
	}
}

func TestParsePathExceptionally(t *testing.T) {
	path := "/r/megaease"
	pattern := "/r/{user"

	match, ret, err := parsePath(path, pattern)
	if err == nil {
		t.Fatalf("expected error unraied %v, %v", match, ret)
	}
}

func TestDuplicatedPathNormally(t *testing.T) {
	path1 := "/r/abc"
	path2 := "/r/def"

	dup, err := duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc"
	path2 = "/r/abc/def"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/"
	path2 = "/r/abc/def"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/defg"
	path2 = "/r/abc/def"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/defg"
	path2 = "/r/abc/def/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/{foo}"
	path2 = "/r/abc/def/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/{foo}/"
	path2 = "/r/abc/def"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/{foo}/"
	path2 = "/r/abc/def/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/def/"
	path2 = "/r/abc/{foo}/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/r/abc/{foo}/{none}"
	path2 = "/r/abc/def/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/test.html"
	path2 = "/{page}"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/{page}"
	path2 = "/test.html"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/{foo}/bar"
	path2 = "/foo/{bar}"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/{foo}/bar/"
	path2 = "/foo/{bar}"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/{foo}/bar"
	path2 = "/foo/{bar}/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/{foo}/bar/baz"
	path2 = "/foo/{bar}/baz"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if !dup {
		t.Fatalf("unexpected check return %v", dup)
	}

	path1 = "/{foo}/bar/{baz}"
	path2 = "/foo/{bar}/"

	dup, err = duplicatedPath(path1, path2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if dup {
		t.Fatalf("unexpected check return %v", dup)
	}
}
