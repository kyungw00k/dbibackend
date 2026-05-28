package protocol

import (
	"fmt"
	"sync"

	"github.com/google/gousb"
)

const (
	SwitchVID = 0x057E
	SwitchPID = 0x3000
)

type USBContext struct {
	ctx    *gousb.Context
	dev    *gousb.Device
	cfg    *gousb.Config
	iface  *gousb.Interface
	out    *gousb.OutEndpoint
	in     *gousb.InEndpoint
	closed bool
	mu     sync.Mutex
}

func ConnectUSB() (usb *USBContext, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("usb init failed: %v", r)
		}
	}()

	ctx := gousb.NewContext()

	dev, err := ctx.OpenDeviceWithVIDPID(SwitchVID, SwitchPID)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("device open failed: %w", err)
	}
	if dev == nil {
		ctx.Close()
		return nil, fmt.Errorf("switch not found (VID:%04x PID:%04x)", SwitchVID, SwitchPID)
	}

	cfgNum, err := dev.ActiveConfigNum()
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("active config: %w", err)
	}

	cfg, err := dev.Config(cfgNum)
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("get config: %w", err)
	}

	iface, err := cfg.Interface(0, 0)
	if err != nil {
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("claim interface: %w", err)
	}

	var inEP, outEP int
	for _, ep := range iface.Setting.Endpoints {
		if ep.Direction == gousb.EndpointDirectionIn {
			inEP = int(ep.Number)
		}
		if ep.Direction == gousb.EndpointDirectionOut {
			outEP = int(ep.Number)
		}
	}

	inEndpoint, err := iface.InEndpoint(inEP)
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("open in endpoint: %w", err)
	}

	outEndpoint, err := iface.OutEndpoint(outEP)
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("open out endpoint: %w", err)
	}

	return &USBContext{
		ctx:   ctx,
		dev:   dev,
		cfg:   cfg,
		iface: iface,
		out:   outEndpoint,
		in:    inEndpoint,
	}, nil
}

func (u *USBContext) Read(buf []byte) (int, error) {
	return u.in.Read(buf)
}

func (u *USBContext) Write(buf []byte) (int, error) {
	return u.out.Write(buf)
}

func (u *USBContext) SendExit() {
	resp, _ := NewHeader(TypeResponse, CmdExit, 0).Marshal()
	u.Write(resp)
}

func (u *USBContext) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return nil
	}
	u.closed = true
	u.iface.Close()
	u.cfg.Close()
	u.dev.Reset()
	u.dev.Close()
	u.ctx.Close()
	return nil
}
