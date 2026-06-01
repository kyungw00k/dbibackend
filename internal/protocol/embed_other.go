//go:build !(darwin || (windows && (amd64 || arm64)))

package protocol

func embeddedLibusbBytes() ([]byte, bool) { return nil, false }
func libusbExt() string                   { return ".so" }
