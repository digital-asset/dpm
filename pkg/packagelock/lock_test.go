package packagelock

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mk(t *testing.T, uris ...string) *PackageLock {
	pl := &PackageLock{}
	for _, u := range uris {
		p, err := url.Parse(u)
		require.NoError(t, err)
		pl.Dars = append(pl.Dars, &Dar{URI: p})
	}
	return pl
}

func TestIsInSync(t *testing.T) {
	tests := []struct {
		name     string
		expected *PackageLock
		existing *PackageLock
		want     bool
	}{
		{
			name:     "no diff",
			expected: mk(t, "oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			existing: mk(t, "oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			want:     true,
		},
		{
			name:     "only removed",
			expected: mk(t, "oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			existing: mk(t, "oci://example1.com/a:latest"),
			want:     false,
		},
		{
			name:     "only added",
			expected: mk(t, "oci://example1.com/a:latest"),
			existing: mk(t, "oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			want:     false,
		},
		{
			name:     "added and removed",
			expected: mk(t, "oci://example1.com/a:latest", "oci://example2.com/b:1.2.3"),
			existing: mk(t, "oci://example2.com/b:1.2.3", "oci://example3.com/c:4.5.6"),
			want:     false,
		},
		{
			name:     "only floaty diff",
			expected: mk(t, "oci://example2.com/b:latest", "builtin://daml-script"),
			existing: mk(t, "oci://example2.com/b:1.2.3", "builtin://daml-script"),
			want:     true,
		},
		{
			name:     "builtin diff",
			expected: mk(t, "builtin://daml-script"),
			existing: mk(t, "builtin://foo"),
			want:     false,
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
