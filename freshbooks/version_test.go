package freshbooks

import "testing"

func TestVersion(t *testing.T) {
	t.Run("[happy] non-empty", func(t *testing.T) {
		if Version == "" {
			t.Fatal("Version must not be empty")
		}
	})
}
