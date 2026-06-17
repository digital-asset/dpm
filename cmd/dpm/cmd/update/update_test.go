package update

import "testing"

func TestExtractTag(t *testing.T) {
	tests := []struct {
		uri string
		tag string
	}{
		{
			"oci://127.0.0.1:50522/newly/added:latest@sha256:a1d6c42f8b80842b71c05152c20fb21e351666b9a07ee0d4e22dfe47ae9a3dbb",
			"latest",
		},
	}
	for _, tt := range tests {
		if got := extractTag(tt.uri); got != tt.tag {
			t.Errorf("ExtractTag(%v) = %v, want %v", tt.uri, got, tt.tag)
		}
	}
}
