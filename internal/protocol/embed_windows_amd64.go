//go:build windows && amd64

package protocol

import _ "embed"

//go:embed lib/windows_amd64.dll
var embeddedLibusb []byte

func embeddedLibusbBytes() ([]byte, bool) { return embeddedLibusb, true }
func libusbExt() string                   { return ".dll" }
