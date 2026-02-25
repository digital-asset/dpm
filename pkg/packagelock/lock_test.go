package packagelock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mk(uris ...string) *PackageLock {
	pl := &PackageLock{}
	for _, u := range uris {
		pl.Dars = append(pl.Dars, &Dar{URI: u})
	}
	return pl
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name     string
		expected *PackageLock
		existing *PackageLock
		want     bool
	}{
		{
			name:     "no diff",
			expected: mk("oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			existing: mk("oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			want:     true,
		},
		{
			name:     "only removed",
			expected: mk("oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			existing: mk("oci://example1.com/a:latest"),
			want:     false,
		},
		{
			name:     "only added",
			expected: mk("oci://example1.com/a:latest"),
			existing: mk("oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			want:     false,
		},
		{
			name:     "added and removed",
			expected: mk("oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			existing: mk("oci://example2.com/b:1.2.3", "oci://example3.com/c:4.5.6"),
			want:     false,
		},
		{
			name:     "only floaty diff",
			expected: mk("oci://example2.com/b:latest"),
			existing: mk("oci://example2.com/b:1.2.3"),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.existing.isInSync(tt.expected)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
