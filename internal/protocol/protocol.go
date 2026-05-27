package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type Header struct {
	Magic    [4]byte
	CmdType  CommandType
	CmdID    CommandID
	DataSize uint32
}

func NewHeader(cmdType CommandType, cmdID CommandID, dataSize uint32) Header {
	return Header{
		Magic:    [4]byte{'D', 'B', 'I', '0'},
		CmdType:  cmdType,
		CmdID:    cmdID,
		DataSize: dataSize,
	}
}

func (h Header) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, h); err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}
	return buf.Bytes(), nil
}

func ReadHeader(r io.Reader) (Header, error) {
	var h Header
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return h, fmt.Errorf("read header: %w", err)
	}
	return h, nil
}

type FileRangeHeader struct {
	RangeSize   uint32
	RangeOffset uint64
	NameLen     uint32
}

func ReadFileRangeHeader(r io.Reader) (FileRangeHeader, string, error) {
	var hdr FileRangeHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr.RangeSize); err != nil {
		return hdr, "", fmt.Errorf("read range size: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.RangeOffset); err != nil {
		return hdr, "", fmt.Errorf("read range offset: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &hdr.NameLen); err != nil {
		return hdr, "", fmt.Errorf("read name len: %w", err)
	}

	nameBuf := make([]byte, hdr.NameLen)
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return hdr, "", fmt.Errorf("read name: %w", err)
	}

	return hdr, string(nameBuf), nil
}
