package fabricx

import "testing"

// TestSlugify_PathTraversalVectors locks in the contract that the
// returned slug never contains a path separator, a parent-directory
// reference, or a NUL — all of which would let a user-supplied node
// name escape the chainlaunch data directory once it flows into
// baseDir → os.MkdirAll / os.WriteFile.
func TestSlugify_PathTraversalVectors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "node"},
		{"only-junk", "..", "node"},
		{"plain", "Acme Committer", "acme-committer"},
		{"already-safe", "acmemsp-committer", "acmemsp-committer"},
		{"underscore-kept", "acme_node_1", "acme_node_1"},
		{"path-traversal", "../../etc/passwd", "etc-passwd"},
		{"absolute-path", "/var/lib/secret", "var-lib-secret"},
		{"backslash", "..\\..\\windows\\system32", "windows-system32"},
		{"nul-byte", "node\x00name", "node-name"},
		{"newline", "node\nname", "node-name"},
		{"collapse-dashes", "a---b___c", "a-b___c"},
		{"unicode-stripped", "café", "caf"},
		{"mixed-case", "MixedCASE", "mixedcase"},
		{"trim-dashes", "---name---", "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := slugify(tc.in)
			if got != tc.want {
				t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// Defense-in-depth assertion: regardless of what we expect
			// in `want`, the slug must NEVER contain any character that
			// makes filepath.Join treat it as a non-trivial path.
			for _, bad := range []string{"/", "\\", "..", "\x00"} {
				if containsSubstring(got, bad) {
					t.Errorf("slugify(%q) = %q contains forbidden token %q", tc.in, got, bad)
				}
			}
		})
	}
}

func containsSubstring(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
