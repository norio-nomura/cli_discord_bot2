package options

import "syscall"

func dup2(oldfd, newfd int) error {
	return syscall.Dup3(oldfd, newfd, 0)
}
