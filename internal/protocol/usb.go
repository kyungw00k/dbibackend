package protocol

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ErrTimeout is returned when a USB bulk transfer times out.
var ErrTimeout = errors.New("usb: transfer timeout")

const (
	SwitchVID = 0x057E
	SwitchPID = 0x3000
)

var (
	fnInit                      func(ctx *unsafe.Pointer) int32
	fnExit                      func(ctx unsafe.Pointer)
	fnOpenDeviceWithVIDPID      func(ctx unsafe.Pointer, vid, pid uint16) unsafe.Pointer
	fnClose                     func(handle unsafe.Pointer)
	fnGetDevice                 func(handle unsafe.Pointer) unsafe.Pointer
	fnGetActiveConfigDescriptor func(dev unsafe.Pointer, config *unsafe.Pointer) int32
	fnFreeConfigDescriptor      func(config unsafe.Pointer)
	fnSetAutoDetachKernelDriver func(handle unsafe.Pointer, enable int32) int32
	fnClaimInterface            func(handle unsafe.Pointer, iface int32) int32
	fnReleaseInterface          func(handle unsafe.Pointer, iface int32) int32
	fnResetDevice               func(handle unsafe.Pointer) int32
	fnBulkTransfer              func(handle unsafe.Pointer, ep uint8, data unsafe.Pointer, length int32, transferred *int32, timeout uint32) int32
)

func init() {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		panic("purego libusb wrapper requires a 64-bit platform")
	}

	lib := dlopenLibusb()
	purego.RegisterLibFunc(&fnInit, lib, "libusb_init")
	purego.RegisterLibFunc(&fnExit, lib, "libusb_exit")
	purego.RegisterLibFunc(&fnOpenDeviceWithVIDPID, lib, "libusb_open_device_with_vid_pid")
	purego.RegisterLibFunc(&fnClose, lib, "libusb_close")
	purego.RegisterLibFunc(&fnGetDevice, lib, "libusb_get_device")
	purego.RegisterLibFunc(&fnGetActiveConfigDescriptor, lib, "libusb_get_active_config_descriptor")
	purego.RegisterLibFunc(&fnFreeConfigDescriptor, lib, "libusb_free_config_descriptor")
	purego.RegisterLibFunc(&fnSetAutoDetachKernelDriver, lib, "libusb_set_auto_detach_kernel_driver")
	purego.RegisterLibFunc(&fnClaimInterface, lib, "libusb_claim_interface")
	purego.RegisterLibFunc(&fnReleaseInterface, lib, "libusb_release_interface")
	purego.RegisterLibFunc(&fnResetDevice, lib, "libusb_reset_device")
	purego.RegisterLibFunc(&fnBulkTransfer, lib, "libusb_bulk_transfer")
}

// USBContext wraps a libusb device connection for the DBI protocol.
type USBContext struct {
	ctx    unsafe.Pointer // libusb_context*
	handle unsafe.Pointer // libusb_device_handle*
	inEP   uint8          // IN endpoint address (e.g. 0x81)
	outEP  uint8          // OUT endpoint address (e.g. 0x01)
	closed bool
	mu     sync.Mutex
}

func ConnectUSB() (usb *USBContext, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("usb init failed: %v", r)
		}
	}()

	var ctx unsafe.Pointer
	if ret := fnInit(&ctx); ret < 0 {
		return nil, fmt.Errorf("libusb_init: error %d", ret)
	}

	handle := fnOpenDeviceWithVIDPID(ctx, SwitchVID, SwitchPID)
	if handle == nil {
		fnExit(ctx)
		return nil, fmt.Errorf("switch not found (VID:%04x PID:%04x)", SwitchVID, SwitchPID)
	}

	dev := fnGetDevice(handle)

	var configDesc unsafe.Pointer
	if ret := fnGetActiveConfigDescriptor(dev, &configDesc); ret < 0 {
		fnClose(handle)
		fnExit(ctx)
		return nil, fmt.Errorf("get active config descriptor: error %d", ret)
	}

	inEP, outEP := findEndpoints(configDesc)
	fnFreeConfigDescriptor(configDesc)

	if inEP == 0 || outEP == 0 {
		fnClose(handle)
		fnExit(ctx)
		return nil, fmt.Errorf("could not find IN/OUT bulk endpoints")
	}

	// Auto-detach kernel driver (harmless on macOS, useful on Linux)
	fnSetAutoDetachKernelDriver(handle, 1)

	if ret := fnClaimInterface(handle, 0); ret < 0 {
		fnClose(handle)
		fnExit(ctx)
		return nil, fmt.Errorf("claim interface: error %d", ret)
	}

	return &USBContext{
		ctx:    ctx,
		handle: handle,
		inEP:   inEP,
		outEP:  outEP,
	}, nil
}

// findEndpoints reads the USB config descriptor to find IN and OUT bulk endpoint addresses.
//
// Struct layouts on 64-bit (from libusb.h):
//
//	libusb_config_descriptor:          40 bytes
//	  offset  4: uint8  bNumInterfaces
//	  offset 16: const libusb_interface *interface
//
//	libusb_interface:                  16 bytes
//	  offset  0: const libusb_interface_descriptor *altsetting
//
//	libusb_interface_descriptor:       40 bytes
//	  offset  4: uint8  bNumEndpoints
//	  offset 16: const libusb_endpoint_descriptor *endpoint
//
//	libusb_endpoint_descriptor:        32 bytes
//	  offset  2: uint8  bEndpointAddress (bit 7 = direction: 1=IN, 0=OUT)
func findEndpoints(configDesc unsafe.Pointer) (inEP, outEP uint8) {
	numIfaces := *(*uint8)(unsafe.Add(configDesc, 4))
	ifaces := *(*unsafe.Pointer)(unsafe.Add(configDesc, 16))

	if numIfaces == 0 {
		return 0, 0
	}

	// Interface 0, altsetting 0
	alt0 := *(*unsafe.Pointer)(unsafe.Add(ifaces, 0))

	numEPs := *(*uint8)(unsafe.Add(alt0, 4))
	eps := *(*unsafe.Pointer)(unsafe.Add(alt0, 16))

	for i := uintptr(0); i < uintptr(numEPs); i++ {
		ep := unsafe.Add(eps, i*32)
		addr := *(*uint8)(unsafe.Add(ep, 2))
		if addr&0x80 != 0 {
			inEP = addr
		} else {
			outEP = addr
		}
	}

	return inEP, outEP
}

func (u *USBContext) Read(buf []byte) (int, error) {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return 0, fmt.Errorf("usb: device closed")
	}
	u.mu.Unlock()

	var transferred int32
	ret := fnBulkTransfer(u.handle, u.inEP, unsafe.Pointer(&buf[0]), int32(len(buf)), &transferred, 1000)
	if ret < 0 {
		if ret == -7 { // LIBUSB_ERROR_TIMEOUT
			return 0, ErrTimeout
		}
		return 0, fmt.Errorf("usb bulk read: error %d", ret)
	}
	return int(transferred), nil
}

func (u *USBContext) Write(buf []byte) (int, error) {
	var transferred int32
	ret := fnBulkTransfer(u.handle, u.outEP, unsafe.Pointer(&buf[0]), int32(len(buf)), &transferred, 0)
	if ret < 0 {
		return 0, fmt.Errorf("usb bulk write: error %d", ret)
	}
	return int(transferred), nil
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

	fnReleaseInterface(u.handle, 0)
	fnResetDevice(u.handle) // Switch disconnect 감지 핵심
	fnClose(u.handle)
	fnExit(u.ctx)
	u.ctx = nil
	u.handle = nil
	return nil
}
