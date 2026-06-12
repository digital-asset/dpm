package testdata

import _ "embed"

//go:embed input.yaml
var Input []byte

//go:embed expected.yaml
var Expected []byte
