package testdata

import _ "embed"

//go:embed non-empty/input.yaml
var InputNonEmptyList []byte

//go:embed non-empty/expected.yaml
var ExpectedNonEmptyList []byte

//go:embed empty/input.yaml
var InputEmptyList []byte

//go:embed empty/expected.yaml
var ExpectedEmptyList []byte
