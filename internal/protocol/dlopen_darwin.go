package protocol

import (
	"fmt"

	"github.com/ebitengine/purego"
)

func dlopenLibusb() uintptr {
	// 1. Embedded library (extracted to cache)
	if path, err := extractEmbeddedLibusb(); err == nil {
		if h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
			return h
		}
	}

	// 2. System path fallback
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
