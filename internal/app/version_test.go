package app

import "testing"

func TestFormatVersionLabel(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "Agent Team Monitor",
			version: "v1.5.0",
			want:    "Agent Team Monitor v1.5.0",
		},
		{
			name:    "Agent Team Monitor",
			version: "1.5.0",
			want:    "Agent Team Monitor v1.5.0",
		},
		{
			name:    "Agent Team Monitor",
			version: "dev",
			want:    "Agent Team Monitor vdev",
		},
		{
			name:    "Agent Team Monitor",
			version: "",
			want:    "Agent Team Monitor",
		},
	}

	for _, tt := range tests {
		if got := FormatVersionLabel(tt.name, tt.version); got != tt.want {
			t.Fatalf("FormatVersionLabel(%q, %q) = %q, want %q", tt.name, tt.version, got, tt.want)
		}
	}
}
