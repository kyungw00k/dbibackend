package protocol

import (
	"fmt"

	"github.com/ebitengine/purego"
)

func dlopenLibusb() uintptr {
	candidates := []string{
		"libusb-1.0.so.0",
		"libusb-1.0.so",
		"/usr/lib/x86_64-linux-gnu/libusb-1.0.so.0",
		"/usr/lib/aarch64-linux-gnu/libusb-1.0.so.0",
	}

	for _, path := range candidates {
		h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return h
		}
	}

	panic(fmt.Sprintf("libusb-1.0 not found (apt install libusb-1.0-0-dev or equivalent)"))
}
