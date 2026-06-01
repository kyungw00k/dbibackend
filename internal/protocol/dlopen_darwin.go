package protocol

import (
	"fmt"

	"github.com/ebitengine/purego"
)

func dlopenLibusb() uintptr {
	candidates := []string{
		"/opt/homebrew/lib/libusb-1.0.dylib",
		"/usr/local/lib/libusb-1.0.dylib",
		"libusb-1.0.dylib",
	}

	for _, path := range candidates {
		h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return h
		}
	}

	panic(fmt.Sprintf("libusb-1.0 not found (brew install libusb)"))
}
