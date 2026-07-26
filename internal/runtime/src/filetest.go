package src

import "os"

// isDir reports whether path names a directory. A path that cannot be
// inspected at all is not a directory, which folds "no such file" and
// "not allowed to look" into the same answer.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isFile reports whether path names an ordinary file: not a directory,
// not a device, not a socket. Symbolic links are followed, so a link to
// a regular file counts.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// fileSize returns the size of path in bytes, and 0 when it cannot be
// inspected. Nothing distinguishes a missing file from an empty one
// here, so check with isFile first where the difference matters.
func fileSize(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(info.Size())
}

// isReadable reports whether path can be opened for reading, which it
// answers by opening it. Reading the permission bits would give a
// different answer: they say what the owner and the group may do, not
// what this process may do.
func isReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isWritable reports whether path has a write bit set for anybody. It is
// an approximation of "this process may write here": it does not know
// which user is running, and it does not consider a read-only mount.
func isWritable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().Perm()&0o222 != 0
}

// isExecutable reports whether path has an execute bit set for anybody,
// with the same approximation isWritable makes.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().Perm()&0o111 != 0
}
