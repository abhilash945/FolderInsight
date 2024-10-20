//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func getFileTimes(path string) (time.Time, time.Time, time.Time, string, error) {
	var ctime, atime, wtime time.Time
	var owner string
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
		if updateWindowsFileOwner {
			owner, err = getFileOwner(path)
			if err != nil {
				errorMSG = fmt.Errorf("cannot get the file owner for %s error message: %v", path, err)
			}
		}
	}
	return ctime, atime, wtime, owner, errorMSG
}

func getFileOwner(path string) (string, error) {
	// Open the file to get its handle
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open %v file, error: %v", path, err)
	}
	defer file.Close()

	// Get the file handle
	handle := windows.Handle(file.Fd())

	// Get security info (Owner SID)
	securityDescriptor, err := windows.GetSecurityInfo(
		handle,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get security info for the file:%v, error: %v", path, err)
	}

	// Get the owner SID from the security descriptor
	ownerSid, _, err := securityDescriptor.Owner()
	if err != nil {
		return "", fmt.Errorf("failed to get owner SID for the file: %v, error: %v", path, err)
	}

	// Allocate buffers to hold the account name and domain name
	var accountNameSize, domainNameSize uint32
	windows.LookupAccountSid(nil, ownerSid, nil, &accountNameSize, nil, &domainNameSize, nil)

	if accountNameSize > 0 && domainNameSize > 0 {
		accountName := make([]uint16, accountNameSize)
		domainName := make([]uint16, domainNameSize)
		var sidType uint32

		// Lookup the account name and domain associated with the SID
		err = windows.LookupAccountSid(nil, ownerSid, &accountName[0], &accountNameSize, &domainName[0], &domainNameSize, &sidType)
		if err != nil {
			return "", fmt.Errorf("failed to lookup account SID: %v", err)
		}

		// Convert account name and domain name from UTF-16 to string
		accountNameStr := windows.UTF16ToString(accountName)
		domainNameStr := windows.UTF16ToString(domainName)

		// Return the full name in "DOMAIN\Account" format
		return fmt.Sprintf("%s\\%s", domainNameStr, accountNameStr), nil
	} else {
		return ownerSid.String(), nil
	}
}
