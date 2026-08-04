package src

import (
	"io/fs"
	"os"
	"syscall"
	"time"
)

// fileStat reports the thirteen numbers a file status call hands back, in
// their traditional order: device, inode, mode, link count, user, group,
// device type, size, access time, modification time, change time, block size
// and block count.
//
// A Go program asks os.Stat for an fs.FileInfo and calls the one method it
// wants: fi.Size(), fi.Mode(), fi.ModTime(), fi.IsDir(). Nothing hands back a
// list, and nothing has to remember which index is which.
func fileStat(path string, follow bool) []int {
	var fi os.FileInfo
	var err error
	if follow {
		fi, err = os.Stat(path)
	} else {
		fi, err = os.Lstat(path)
	}
	if err != nil {
		return nil
	}
	out := make([]int, 13)
	out[2] = int(fi.Mode().Perm()) | modeBits(fi.Mode())
	out[7] = int(fi.Size())
	out[9] = int(fi.ModTime().Unix())
	out[8], out[10] = out[9], out[9]
	out[3] = 1
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		out[0] = int(sys.Dev)
		out[1] = int(sys.Ino)
		out[3] = int(sys.Nlink)
		out[4] = int(sys.Uid)
		out[5] = int(sys.Gid)
		out[6] = int(sys.Rdev)
		out[8] = int(sys.Atim.Sec)
		out[10] = int(sys.Ctim.Sec)
		out[11] = int(sys.Blksize)
		out[12] = int(sys.Blocks)
	}
	return out
}

// modeBits turns the type part of a Go file mode back into the bits a status
// call reports, which is where the difference between a file, a directory and
// a link lives.
func modeBits(mode fs.FileMode) int {
	switch {
	case mode&fs.ModeDir != 0:
		return 0o040000
	case mode&fs.ModeSymlink != 0:
		return 0o120000
	case mode&fs.ModeNamedPipe != 0:
		return 0o010000
	case mode&fs.ModeSocket != 0:
		return 0o140000
	case mode&fs.ModeDevice != 0:
		return 0o060000
	}
	return 0o100000
}

// isLink reports whether a path is a symbolic link, which needs the call that
// does not follow one: os.Stat would answer about the target instead.
func isLink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&fs.ModeSymlink != 0
}

// setMode changes a path's permission bits and reports how many paths it
// changed, which is what the caller counted.
func setMode(mode int, paths ...string) int {
	changed := 0
	for _, p := range paths {
		if os.Chmod(p, fs.FileMode(mode)) == nil {
			changed++
		}
	}
	return changed
}

// setTimes sets a path's access and modification times from whole seconds.
func setTimes(atime, mtime int, paths ...string) int {
	changed := 0
	for _, p := range paths {
		if os.Chtimes(p, time.Unix(int64(atime), 0), time.Unix(int64(mtime), 0)) == nil {
			changed++
		}
	}
	return changed
}

// readLink reports where a symbolic link points, and "" when the path is not
// a link at all.
//
// os.Readlink returns the target and an error, and a path that is not a link
// is an error rather than an empty answer, which is the more useful of the
// two behaviours and the one to keep.
func readLink(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

// removeFiles deletes each path and reports how many went, which is what the
// caller counted.
//
// os.Remove takes one path and returns an error, and a path that was not
// there is an error rather than a zero. Counting is the older shape and loses
// the reason, which is why a Go program tests the error instead.
func removeFiles(paths ...string) int {
	gone := 0
	for _, p := range paths {
		if os.Remove(p) == nil {
			gone++
		}
	}
	return gone
}
