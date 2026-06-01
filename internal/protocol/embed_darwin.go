//go:build darwin

package protocol

import _ "embed"

//go:embed lib/darwin_universal.dylib
var embeddedLibusb []byte

func embeddedLibusbBytes() ([]byte, bool) { return embeddedLibusb, true }
func libusbExt() string                   { return ".dylib" }
