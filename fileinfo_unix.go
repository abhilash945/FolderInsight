//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

// returns ctime, atime, wtime, uid:gid, errorMSG of a file
func getFileTimes(path string) (time.Time, time.Time, time.Time, string, error) {
	var ctime, atime, wtime time.Time
	var uid_gid string
	var errorMSG error

	info, err := os.Stat(path)
	if err != nil {
		errorMSG = fmt.Errorf("cannot get the non-windows stat for %s error message: %v", path, err)
	} else {
		stat := info.Sys().(*unix.Stat_t)
		uid_gid = strconv.FormatUint(uint64(stat.Uid), 10) + ":" + strconv.FormatUint(uint64(stat.Gid), 10)
		atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		wtime = info.ModTime()
		ctime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
		errorMSG = nil
	}
	return ctime, atime, wtime, uid_gid, errorMSG
}
