//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestDesktopX11SocketPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		display string
		want    string
	}{
		{name: "empty", display: "", want: ""},
		{name: "wayland", display: "wayland-0", want: ""},
		{name: "local display", display: ":0", want: "@/tmp/.X11-unix/X0"},
		{name: "screen suffix", display: ":12.0", want: "@/tmp/.X11-unix/X12"},
		{name: "invalid local display", display: ":", want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := desktopX11SocketPath(tc.display); got != tc.want {
				t.Fatalf("desktopX11SocketPath(%q) = %q, want %q", tc.display, got, tc.want)
			}
		})
	}
}

func TestCountUnixSocketEntriesInReader(t *testing.T) {
	t.Parallel()

	data := strings.NewReader(`Num       RefCount Protocol Flags    Type St Inode Path
0000000000000000: 00000003 00000000 00000000 0001 03 9947117 @/tmp/.X11-unix/X0
0000000000000000: 00000003 00000000 00000000 0001 03 124166 @/tmp/.X11-unix/X0
0000000000000000: 00000003 00000000 00000000 0001 03 9790883 @/tmp/.X11-unix/X1
`)

	got, err := countUnixSocketEntriesInReader(data, "@/tmp/.X11-unix/X0")
	if err != nil {
		t.Fatalf("countUnixSocketEntriesInReader returned error: %v", err)
	}
	if got != 2 {
		t.Fatalf("countUnixSocketEntriesInReader returned %d, want 2", got)
	}
}
