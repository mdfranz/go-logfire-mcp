package main

import "testing"

func TestDebugEnabled(t *testing.T) {
	for _, value := range []string{"", "true", "1", "unexpected"} {
		if !debugEnabled(value) {
			t.Errorf("debugEnabled(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"0", "false", "off", "FALSE"} {
		if debugEnabled(value) {
			t.Errorf("debugEnabled(%q) = true, want false", value)
		}
	}
}
