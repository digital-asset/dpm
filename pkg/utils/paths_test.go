package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUrlToFilePath(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{
			in:       "foo.com/public-unstable/foo",
			expected: "foo.com/public-unstable/foo",
		},
		{
			in:       "localhost:5000/foo/bar",
			expected: "localhost_5000/foo/bar",
		},
		{
			in:       `localhost:5000/foo bar/baz`,
			expected: "localhost_5000/foo_bar/baz",
		},
		{
			in:       `127.0.0.1:5000/foo bar/baz`,
			expected: "127.0.0.1_5000/foo_bar/baz",
		},
		{
			in:       "ghcr.io/foo./bar",
			expected: "ghcr.io/foo/bar",
		},
		{
			in:       "ghcr.io/./bar",
			expected: "ghcr.io/_/bar",
		},
		{
			in:       "foo.com:8080/a-b/c:d",
			expected: "foo.com_8080/a-b/c_d",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, UrlToFilePath(tt.in))
	}
}
