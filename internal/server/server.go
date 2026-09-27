package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/kyungw00k/dbibackend/internal/protocol"
)

type usbReader struct {
	usb *protocol.USBContext
}

func (r usbReader) Read(p []byte) (int, error) {
	for {
		n, err := r.usb.Read(p)
		if errors.Is(err, protocol.ErrTimeout) {
			continue
		}
		return n, err
	}
}

type Server struct {
	usb     *protocol.USBContext
	workDir string
	paths   []string
	cache   map[string]string
	logger  *slog.Logger
	stop    chan struct{}
}

func New(usb *protocol.USBContext, workDir string, logger *slog.Logger) *Server {
	return &Server{
		usb:     usb,
		workDir: workDir,
		paths:   []string{workDir},
		cache:   make(map[string]string),
		logger:  logger,
		stop:    make(chan struct{}),
	}
}

func NewMulti(usb *protocol.USBContext, paths []string, logger *slog.Logger) *Server {
	dir := ""
	if len(paths) > 0 {
		dir = paths[0]
	}
	return &Server{
		usb:     usb,
		workDir: dir,
		paths:   paths,
		cache:   make(map[string]string),
		logger:  logger,
		stop:    make(chan struct{}),
	}
}

func (s *Server) Stop() {
	close(s.stop)
}

func (s *Server) RunWithContext(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return s.Run()
}

func (s *Server) reader() io.Reader {
	return usbReader{usb: s.usb}
}

func (s *Server) Run() error {
	s.logger.Info("entering command loop")
	for {
		select {
		case <-s.stop:
			s.logger.Info("stop requested, sending exit")
			return s.handleExit()
		default:
		}

		headerBuf := make([]byte, 16)

		type readResult struct {
			n   int
			err error
		}
		ch := make(chan readResult, 1)
		go func() {
			n, err := io.ReadFull(s.reader(), headerBuf)
			ch <- readResult{n, err}
		}()

		var res readResult
		select {
		case <-s.stop:
			s.logger.Info("stop requested, shutting down")
			// Close USB directly — fnResetDevice causes the Switch to detect
			// disconnect. Do NOT try to Write an exit command first: Write
			// uses timeout=0 and will block if the Switch isn't actively reading.
			s.usb.Close()
			return fmt.Errorf("stopped")
		case res = <-ch:
		}

		if res.err != nil {
			return fmt.Errorf("read header: %w", res.err)
		}

		header, err := protocol.ReadHeader(bytes.NewReader(headerBuf))
		if err != nil {
			return fmt.Errorf("parse header: %w", err)
		}

		if string(header.Magic[:]) != protocol.Magic {
			continue
		}

		s.logger.Debug("command received",
			"type", header.CmdType,
			"id", header.CmdID,
			"size", header.DataSize,
		)

		switch header.CmdID {
		case protocol.CmdExit:
			return s.handleExit()
		case protocol.CmdList:
			if err := s.handleList(); err != nil {
				return fmt.Errorf("handle list: %w", err)
			}
		case protocol.CmdFileRange:
			if err := s.handleFileRange(header.DataSize); err != nil {
				return fmt.Errorf("handle file range: %w", err)
			}
		default:
			s.logger.Warn("unknown command", "id", header.CmdID)
			return s.handleExit()
		}
	}
}

func (s *Server) handleExit() error {
	s.logger.Info("exit")
	resp, _ := protocol.NewHeader(protocol.TypeResponse, protocol.CmdExit, 0).Marshal()
	s.usb.Write(resp)
	return fmt.Errorf("switch requested exit")
}

func (s *Server) handleList() error {
	s.logger.Info("get list")
	s.cache = make(map[string]string)

	extensions := map[string]bool{".nsp": true, ".nsz": true, ".xci": true}
	var names []string

	for _, dir := range s.paths {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if extensions[strings.ToLower(filepath.Ext(path))] {
				displayName := norm.NFC.String(d.Name())
				if _, dup := s.cache[displayName]; dup {
					return nil // same title already listed from another path
				}
				s.cache[displayName] = path
				names = append(names, displayName)
			}
			return nil
		})
	}

	listData := []byte(strings.Join(names, "\n") + "\n")
	resp, _ := protocol.NewHeader(protocol.TypeResponse, protocol.CmdList, uint32(len(listData))).Marshal()
	s.usb.Write(resp)

	ackBuf := make([]byte, 16)
	io.ReadFull(s.reader(), ackBuf)
	s.logger.Debug("ack received")

	_, err := s.usb.Write(listData)
	return err
}

func (s *Server) handleFileRange(dataSize uint32) error {
	s.logger.Debug("file range")

	ack, _ := protocol.NewHeader(protocol.TypeAck, protocol.CmdFileRange, dataSize).Marshal()
	s.usb.Write(ack)

	rangeData := make([]byte, dataSize)
	if _, err := io.ReadFull(s.reader(), rangeData); err != nil {
		return fmt.Errorf("read range header: %w", err)
	}

	rangeSize := binary.LittleEndian.Uint32(rangeData[0:4])
	rangeOffset := binary.LittleEndian.Uint64(rangeData[4:12])
	nameLen := binary.LittleEndian.Uint32(rangeData[12:16])
	nspName := string(rangeData[16 : 16+nameLen])

	if resolved, ok := s.cache[nspName]; ok {
		nspName = resolved
	}

	s.logger.Debug("range info",
		"size", rangeSize,
		"offset", rangeOffset,
		"name", nspName,
	)

	resp, _ := protocol.NewHeader(protocol.TypeResponse, protocol.CmdFileRange, rangeSize).Marshal()
	s.usb.Write(resp)

	ackBuf := make([]byte, 16)
	io.ReadFull(s.reader(), ackBuf)

	f, err := os.Open(nspName)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	f.Seek(int64(rangeOffset), io.SeekStart)

	buf := make([]byte, protocol.BufferSegmentDataSize)
	remaining := int64(rangeSize)

	for remaining > 0 {
		chunk := int64(len(buf))
		if remaining < chunk {
			chunk = remaining
		}

		n, err := f.Read(buf[:chunk])
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		if _, err := s.usb.Write(buf[:n]); err != nil {
			return fmt.Errorf("write usb: %w", err)
		}

		remaining -= int64(n)
	}

	return nil
}

func WaitForSwitch(logger *slog.Logger) (*protocol.USBContext, error) {
	return WaitForSwitchCtx(context.Background(), logger)
}

func WaitForSwitchCtx(ctx context.Context, logger *slog.Logger) (*protocol.USBContext, error) {
	loggedWaiting := false
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		usb, err := protocol.ConnectUSB()
		if err == nil {
			return usb, nil
		}
		if !loggedWaiting {
			logger.Info("waiting for switch")
			loggedWaiting = true
		}
		logger.Debug("waiting for switch", "err", err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}
