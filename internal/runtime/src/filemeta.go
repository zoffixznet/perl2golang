package src

import (
	"io/fs"
	"os"
	"reflect"
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
//
// Mode, size and modification time come from that fs.FileInfo and are the same
// everywhere. The rest live in the operating system's own status record, which
// fi.Sys() returns as a different type on every system, so they are read by
// field name and left alone where the running system has no field by that
// name. On Windows most of them are, which is the same answer the original
// gives there.
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
	sys := reflect.Indirect(reflect.ValueOf(fi.Sys()))
	if sys.Kind() != reflect.Struct {
		return out
	}
	statField(sys, "Dev", &out[0])
	statField(sys, "Ino", &out[1])
	statField(sys, "Nlink", &out[3])
	statField(sys, "Uid", &out[4])
	statField(sys, "Gid", &out[5])
	statField(sys, "Rdev", &out[6])
	statTime(sys, &out[8], "Atim", "Atimespec")
	statTime(sys, &out[10], "Ctim", "Ctimespec")
	statField(sys, "Blksize", &out[11])
	statField(sys, "Blocks", &out[12])
	return out
}

// statField copies one whole number out of the system's status record into
// dst, and leaves dst as it found it when there is no field by that name or it
// holds something other than a number.
func statField(sys reflect.Value, name string, dst *int) {
	f := sys.FieldByName(name)
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*dst = int(f.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*dst = int(f.Uint())
	}
}

// statTime copies the whole seconds of one timestamp into dst, trying each
// spelling in turn because the systems disagree about the name: a timestamp is
// Atim on Linux and Atimespec on macOS and the BSDs.
func statTime(sys reflect.Value, dst *int, names ...string) {
	for _, name := range names {
		if ts := sys.FieldByName(name); ts.Kind() == reflect.Struct {
			statField(ts, "Sec", dst)
			return
		}
	}
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
