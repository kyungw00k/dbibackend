package protocol

import (
	"fmt"
	"syscall"
)

func dlopenLibusb() uintptr {
	candidates := []string{
		"libusb-1.0.dll",
	}

	for _, path := range candidates {
		h, err := syscall.LoadLibrary(path)
		if err == nil {
			return uintptr(h)
		}
	}

	panic(fmt.Sprintf("libusb-1.0 not found (download from libusb.info and place libusb-1.0.dll in PATH)"))
}
