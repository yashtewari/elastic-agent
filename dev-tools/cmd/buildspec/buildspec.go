// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"bytes"
	"flag"
	"fmt"

	"go/format"
	"io/ioutil"
	"os"
	"text/template"

	lic "github.com/elastic/elastic-agent-poc/dev-tools/licenses"
	"github.com/elastic/elastic-agent-poc/internal/pkg/packer"
)

var (
	input   string
	output  string
	license string
)

func init() {
	flag.StringVar(&input, "in", "", "Source of input. \"-\" means reading from stdin")
	//flag.StringVar(&output, "out", "-", "Output path. \"-\" means writing to stdout")
	flag.StringVar(&license, "license", "Elastic", "License header for generated file.")
}

var tmpl = template.Must(template.New("specs").Parse(`
{{ .License }}
// Code generated by elastic-agent/internals/dev-tools/buildspec/buildspec.go - DO NOT EDIT.

package program

import (
	"strings"

	"github.com/elastic/elastic-agent-poc/internal/pkg/packer"
)

var Supported []Spec
var SupportedMap map[string]Spec

func init() {
	// Packed Files
	{{ range $i, $f := .Files -}}
	// {{ $f }}
	{{ end -}}
	unpacked := packer.MustUnpack("{{ .Pack }}")
	SupportedMap = make(map[string]Spec)

	for f, v := range unpacked {
	s, err:= NewSpecFromBytes(v)
		if err != nil {
			panic("Cannot read spec from " + f)
		}
		Supported = append(Supported, s)
		SupportedMap[strings.ToLower(s.Cmd)] = s
	}
}
`))

func main() {
	flag.Parse()

	if len(input) == 0 {
		fmt.Fprintln(os.Stderr, "Invalid input source")
		os.Exit(1)
	}

	l, err := lic.Find(license)
	if err != nil {
		fmt.Fprintf(os.Stderr, "problem to retrieve the license, error: %+v", err)
		os.Exit(1)
		return
	}

	data, err := gen(input, l)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while generating the file, err: %+v\n", err)
		os.Exit(1)
	}

	if output == "-" {
		os.Stdout.Write(data)
		return
	} else {
		ioutil.WriteFile(output, data, 0640)
	}

	return
}

func gen(path string, l string) ([]byte, error) {
	pack, files, err := packer.Pack(input)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tmpl.Execute(&buf, struct {
		Pack    string
		Files   []string
		License string
	}{
		Pack:    pack,
		Files:   files,
		License: l,
	})

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return formatted, nil
}
