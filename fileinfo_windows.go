//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func getFileTimes(path string) (time.Time, time.Time, time.Time, error) {
	var ctime, atime, wtime time.Time
	var errorMSG error

	info, err := os.Stat(path)
	if err != nil {
		errorMSG = fmt.Errorf("cannot get the windows stat for %s error message: %v", path, err)
	} else {
		sys := info.Sys()
		winSys := sys.(*syscall.Win32FileAttributeData)
		ctime = time.Unix(0, winSys.CreationTime.Nanoseconds())
		atime = time.Unix(0, winSys.LastAccessTime.Nanoseconds())
		wtime = time.Unix(0, winSys.LastWriteTime.Nanoseconds())
		errorMSG = nil
	}
	return ctime, atime, wtime, errorMSG
}
