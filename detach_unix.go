//go:build !windows

package main

import "syscall"

// detachAttr puts the background distiller in its own process group. Ignoring
// signals is not enough on its own: a supervisor that tears down the session by
// killing the whole group takes an inherited-group child with it regardless of
// what that child ignores.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
