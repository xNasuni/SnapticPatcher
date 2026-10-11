package main

import (
	"os"
	"syscall"
	"unsafe"
)

func enableColor() {
	h := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	k := syscall.NewLazyDLL("kernel32.dll")
	if r, _, _ := k.NewProc("GetConsoleMode").Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r != 0 {
		k.NewProc("SetConsoleMode").Call(uintptr(h), uintptr(mode|0x4))
	}
}
