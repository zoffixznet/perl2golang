package src

import (
	"os"
	"path/filepath"
)

// programPath is the path of the running binary, or the name it was invoked
// under when the system cannot say.
//
// os.Executable returns an error because not every system can answer, and it
// reports where the binary is rather than where any source file was.
func programPath() string {
	path, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return path
}

// programDir is the directory the running binary sits in.
func programDir() string {
	return filepath.Dir(programPath())
}
