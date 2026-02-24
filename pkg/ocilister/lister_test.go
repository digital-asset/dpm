package ocilister

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFloaty(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"3.4", true},
		{"latest", true},
		{"3", true},
		{"3.4.0-rc2.darwin_amd64", true},
		{"3.4.0.generic", true},
		{"1.0.10-snapshot-20260107.14.0.v70c7b3c.darwin_amd64", true},

		{"3.5.0-snapshot.20260220.0.4b3a869", false},
		{"3.4.0", false},
		{"3.4.0-rc2", false},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			assert.Equal(t, tc.want, IsFloaty(tc.tag))
		})
	}
}

func TestIsPlatformTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"3.4", false},
		{"latest", false},
		{"3", false},
		{"3.4.0", false},
		{"3.4.0-hello", false},
		{"3.4.0-rc2.darwin_amd_64", false},
		{"3.4.generic", false},
		{"3.5.0-snapshot.20260220.0.4b3a869", false},

		{"1.0.10-snapshot-20260107.14.0.v70c7b3c.darwin_amd64", true},
		{"3.4.0-rc2.darwin_amd64", true},
		{"3.4.0-rc2.generic", true},
		{"3.4.0.generic", true},
		{"3.4.0.darwin_amd64", true},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			assert.Equal(t, tc.want, IsPlatformTag(tc.tag))
		})
	}
}
