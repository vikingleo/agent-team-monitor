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

func TestReadBoolEnv(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		want         bool
	}{
		{
			name:         "empty uses default",
			value:        "",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "standard true",
			value:        "true",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "alias yes",
			value:        "yes",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "alias no",
			value:        "no",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "invalid uses default",
			value:        "invalid",
			defaultValue: false,
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				t.Setenv("ATM_BOOL_TEST", "")
			} else {
				t.Setenv("ATM_BOOL_TEST", tc.value)
			}

			got := readBoolEnv("ATM_BOOL_TEST", tc.defaultValue)
			if got != tc.want {
				t.Fatalf("readBoolEnv got %v, want %v", got, tc.want)
			}
		})
	}
}
