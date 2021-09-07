package command

import (
	"bufio"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// VisitorFunc executes visition logic
type VisitorFunc func([]spec)

// Visitor walk through the document via VisitorFunc
type Visitor interface {
	Visit(VisitorFunc)
}

type spec struct {
	Kind string
	Name string
	doc  string
}

type streamVisitor struct {
	io.Reader
}

// NewStreamVisitor returns a streamVisitor.
func NewStreamVisitor(src string) *streamVisitor {
	return &streamVisitor{
		Reader: strings.NewReader(src),
	}
}

type yamlDecoder struct {
	reader *yaml.YAMLReader
	doc    string
}

func newYAMLDecoder(r io.Reader) *yamlDecoder {
	return &yamlDecoder{
		reader: yaml.NewYAMLReader(bufio.NewReader(r)),
	}
}

// Decode reads a YAML document into bytes and tries to yaml.Unmarshal it.
func (d *yamlDecoder) Decode(into interface{}) error {
	bytes, err := d.reader.Read()
	if err != nil && err != io.EOF {
		return err
	}
	d.doc = string(bytes)
	if len(bytes) != 0 {
		err = yaml.Unmarshal(bytes, into)
	}
	return err
}

// Visit implements Visitor over a stream.
func (v *streamVisitor) Visit(fn VisitorFunc) {
	d := newYAMLDecoder(v.Reader)
	var validDocs []spec
	for {
		var s spec
		if err := d.Decode(&s); err != nil {
			if err == io.EOF {
				break
			} else {
				ExitWithErrorf("error parsing %s: %v", d.doc, err)
			}
		}
		s.doc = d.doc
		//TODO can validate spec's Kind here
		validDocs = append(validDocs, s)
	}
	fn(validDocs)
}
