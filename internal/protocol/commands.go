package protocol

const (
	Magic = "DBI0"

	BufferSegmentDataSize = 0x100000
)

type CommandID uint32

const (
	CmdExit          CommandID = 0
	CmdListDeprecated CommandID = 1
	CmdFileRange     CommandID = 2
	CmdList          CommandID = 3
)

type CommandType uint32

const (
	TypeRequest  CommandType = 0
	TypeResponse CommandType = 1
	TypeAck      CommandType = 2
)
