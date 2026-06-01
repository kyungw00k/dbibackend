//go:build windows && arm64

package protocol

import _ "embed"

//go:embed lib/windows_arm64.dll
var embeddedLibusb []byte

func embeddedLibusbBytes() ([]byte, bool) { return embeddedLibusb, true }
func libusbExt() string                   { return ".dll" }
