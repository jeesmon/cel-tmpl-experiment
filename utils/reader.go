package utils

import (
	"io/ioutil"

	"github.com/google/cel-policy-templates-go/policy/model"
)

// NewReader constructs a new test reader with the relative location of the testdata.
func NewReader(relDir string) *reader {
	return &reader{relDir: relDir}
}

type reader struct {
	relDir string
}

// Read returns the Source instance for the given file name.
func (r *reader) Read(fileName string) (*model.Source, bool) {
	tmplBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, false
	}
	return model.ByteSource(tmplBytes, fileName), true
}
