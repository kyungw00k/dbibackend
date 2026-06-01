package protocol

import (
	"fmt"
	"syscall"
)

func dlopenLibusb() uintptr {
	// 1. Embedded library (extracted to cache)
	if path, err := extractEmbeddedLibusb(); err == nil {
		if h, err := syscall.LoadLibrary(path); err == nil {
			return uintptr(h)
		}
	}

	// 2. System path fallback
	h, err := syscall.LoadLibrary("libusb-1.0.dll")
	if err == nil {
		return uintptr(h)
	}

	panic(fmt.Sprintf("libusb-1.0 not found (download from libusb.info and place libusb-1.0.dll in PATH)"))
}
