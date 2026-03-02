package sysproxy

import "testing"

func TestDetectLinuxDesktopEnvironmentFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want DesktopEnvironment
	}{
		{
			name: "gnome from xdg",
			env: map[string]string{
				"XDG_CURRENT_DESKTOP": "GNOME",
			},
			want: DesktopGNOME,
		},
		{
			name: "gnome from session id",
			env: map[string]string{
				"GNOME_DESKTOP_SESSION_ID": "this-is-gnome",
			},
			want: DesktopGNOME,
		},
		{
			name: "kde from xdg",
			env: map[string]string{
				"XDG_CURRENT_DESKTOP": "KDE",
			},
			want: DesktopKDE,
		},
		{
			name: "kde from plasma session",
			env: map[string]string{
				"DESKTOP_SESSION": "plasma",
			},
			want: DesktopKDE,
		},
		{
			name: "unknown desktop",
			env: map[string]string{
				"XDG_CURRENT_DESKTOP": "xfce",
				"DESKTOP_SESSION":     "xfce",
			},
			want: DesktopUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(k string) string {
				return tc.env[k]
			}
			if got := detectLinuxDesktopEnvironmentFromEnv(getenv); got != tc.want {
				t.Fatalf("detectLinuxDesktopEnvironmentFromEnv()=%q want=%q", got, tc.want)
			}
		})
	}
}
