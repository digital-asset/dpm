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

//go:embed replace/last/input.yaml
var InputReplaceLast []byte

//go:embed replace/last/expected.yaml
var ExpectedReplaceLast []byte

//go:embed replace/not-last/input.yaml
var InputReplaceNotLast []byte

//go:embed replace/not-last/expected.yaml
var ExpectedReplaceNotLast []byte
