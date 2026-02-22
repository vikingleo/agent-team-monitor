package monitor

import "testing"

func TestSanitizeDisplayPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "home path",
			input: "/home/tester/work/project-a",
			want:  "~/work/project-a",
		},
		{
			name:  "non-home absolute path",
			input: "/opt/services/project-b",
			want:  "project-b",
		},
		{
			name:  "windows absolute path",
			input: `C:\Users\alice\project-c`,
			want:  "project-c",
		},
		{
			name:  "relative path",
			input: "workspace/project-d",
			want:  "workspace/project-d",
		},
		{
			name:  "empty path",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDisplayPath(tc.input)
			if got != tc.want {
				t.Fatalf("sanitizeDisplayPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
