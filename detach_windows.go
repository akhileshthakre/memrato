//go:build windows

package main

import "syscall"

// detachAttr is the Windows spelling of detach_unix.go's Setpgid: a new console
// process group, so a Ctrl+C or a group teardown aimed at the session does not
// reach the background distiller.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
