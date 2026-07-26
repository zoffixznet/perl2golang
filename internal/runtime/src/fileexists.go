package src

import "os"

// fileExists reports whether path names something that can be
// inspected. Symbolic links are followed, so a link whose target is gone
// does not exist.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
