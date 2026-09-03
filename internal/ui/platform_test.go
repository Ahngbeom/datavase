package ui

import "testing"

// tmux never forwards ⌘ to the application it hosts, so on a Mac inside
// tmux the interface has to name the Ctrl form — the one binding that can
// actually arrive — rather than a glyph the user can press forever and
// never see take effect.
func TestMacLabelsStepsAsideInsideTmux(t *testing.T) {
	tests := []struct {
		name string
		goos string
		tmux string
		want bool
	}{
		{name: "mac, no tmux", goos: "darwin", tmux: "", want: true},
		{name: "mac, inside tmux", goos: "darwin", tmux: "/tmp/tmux-501/default,1234,0", want: false},
		{name: "not mac, no tmux", goos: "linux", tmux: "", want: false},
		{name: "not mac, inside tmux", goos: "linux", tmux: "/tmp/tmux-501/default,1234,0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := macLabels(tt.goos, tt.tmux); got != tt.want {
				t.Errorf("macLabels(%q, %q) = %v, want %v", tt.goos, tt.tmux, got, tt.want)
			}
		})
	}
}
