//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func getFileTimes(path string) (time.Time, time.Time, time.Time, error) {
	var ctime, atime, wtime time.Time
	var errorMSG error

	info, err := os.Stat(path)
	if err != nil {
		errorMSG = fmt.Errorf("cannot get the non-windows stat for %s error message: %v", path, err)
	} else {
		stat := info.Sys().(*unix.Stat_t)
		atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		wtime = info.ModTime()
		ctime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
		errorMSG = nil
	}
	return ctime, atime, wtime, errorMSG
}
