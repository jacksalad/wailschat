//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

const spiGetWorkArea = 0x0030

type rect struct {
	Left, Top, Right, Bottom int32
}

// getWorkArea returns the usable screen rectangle (left, top, right, bottom)
// excluding the taskbar and other desktop toolbars.
func getWorkArea() (int, int, int, int) {
	var rc rect
	procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&rc)), 0)
	return int(rc.Left), int(rc.Top), int(rc.Right), int(rc.Bottom)
}
