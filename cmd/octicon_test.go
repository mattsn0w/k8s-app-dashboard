package main

import "testing"

// TestIsValidOcticonName exercises the strict Primer Octicon name validator used
// for the workload-icon deployment annotation.
func TestIsValidOcticonName(t *testing.T) {
	valid := []string{
		"rocket", "cloud", "cloud-offline", "issue-reopened",
		"crosshairs", "link-external", "terminal", "server", "vscode",
		"gear", "pulse", "beaker", "webhook", "log", "moon", "sun",
		"1password", "vscode16", "cloud-offline-fill",
	}
	for _, v := range valid {
		if !isValidOcticonName(v) {
			t.Errorf("isValidOcticonName(%q) = false, want true", v)
		}
	}

	invalid := []string{
		"",                          // empty
		"-leading",                  // leading hyphen
		"trailing-",                 // trailing hyphen
		"double--hyphen",            // consecutive hyphens
		"UPPER",                     // uppercase
		"MiXeD",                     // mixed case
		"under_score",               // underscore
		"space name",                // space
		"dash/dash",                 // slash
		"a b-c",                     // space + hyphen
		"rocket\n",                  // newline
		"emoji\u2764",                // non-ascii symbol
		"tab\tname",                 // tab
	}
	for _, v := range invalid {
		if isValidOcticonName(v) {
			t.Errorf("isValidOcticonName(%q) = true, want false", v)
		}
	}

	// Length guard: > 64 chars is invalid even if it matches the pattern.
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	if isValidOcticonName(string(long)) {
		t.Errorf("isValidOcticonName(65-char name) = true, want false")
	}
}
