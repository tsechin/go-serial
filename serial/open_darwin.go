// Copyright 2011 Aaron Jacobs. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file contains OS-specific constants and types that work on OS X (tested
// on version 10.6.8).
//
// Helpful documentation for some of these options:
//
//     http://www.unixwiz.net/techtips/termios-vmin-vtime.html
//     http://www.taltech.com/support/entry/serial_intro
//

package serial

import "io"
import "os"
import "syscall"
import "unsafe"

// termios types
type cc_t byte
type speed_t uint64
type tcflag_t uint64

// sys/termios.h
const (
	CS5    = 0x00000000
	CS6    = 0x00000100
	CS7    = 0x00000200
	CS8    = 0x00000300
	CLOCAL = 0x00008000
	CREAD  = 0x00000800
	IGNPAR = 0x00000004

	NCCS = 20

	VMIN  = tcflag_t(16)
	VTIME = tcflag_t(17)
)

// sys/ttycom.h
const (
	TIOCGETA = 1078490131
	TIOCSETA = 2152231956
)

// sys/termios.h
type termios struct {
	c_iflag  tcflag_t
	c_oflag  tcflag_t
	c_cflag  tcflag_t
	c_lflag  tcflag_t
	c_cc     [NCCS]cc_t
	c_ispeed speed_t
	c_ospeed speed_t
}

// setTermios updates the termios struct associated with a serial port file
// descriptor. This sets appropriate options for how the OS interacts with the
// port.
func setTermios(fd int, src *termios) os.Error {
	// Make the ioctl syscall that sets the termios struct.
	r1, _, errno :=
		syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(fd),
			uintptr(TIOCSETA),
			uintptr(unsafe.Pointer(src)))

	// Did the syscall return an error?
	if err := os.NewSyscallError("SYS_IOCTL", int(errno)); err != nil {
		return err
	}

	// Just in case, check the return value as well.
	if r1 != 0 {
		return os.NewError("Unknown error from SYS_IOCTL.")
	}

	return nil
}

func convertOptions(options OpenOptions) (*termios, os.Error) {
	var result termios

	// Ignore modem status lines. We don't want to receive SIGHUP when the serial
	// port is disconnected, for example.
	result.c_cflag |= CLOCAL

	// Enable receiving data.
	//
	// NOTE(jacobsa): I don't know exactly what this flag is for. The man page
	// seems to imply that it shouldn't really exist.
	result.c_cflag |= CREAD

	// Ignore parity errors.
	//
	// TODO(jacobsa): Make this an option instead.
	result.c_iflag |= IGNPAR

	// Turn off the inter-character timer.
	//
	// TODO(jacobsa): Make this an option instead.
	result.c_cc[VTIME] = 0

	// Make reads block until one byte is received.
	//
	// TODO(jacobsa): Make this an option instead.
	result.c_cc[VMIN] = 1

	// Baud rate
	switch options.BaudRate {
	case 50:
	case 75:
	case 110:
	case 134:
	case 150:
	case 200:
	case 300:
	case 600:
	case 1200:
	case 1800:
	case 2400:
	case 4800:
	case 7200:
	case 9600:
	case 14400:
	case 19200:
	case 28800:
	case 38400:
	case 57600:
	case 76800:
	case 115200:
	case 230400:
	default:
		return nil, os.NewError("Invalid setting for BaudRate.")
	}

	// On OS X, the termios.h constants for speeds just map to the values
	// themselves.
	result.c_ispeed = speed_t(options.BaudRate)
	result.c_ospeed = speed_t(options.BaudRate)

	// Data bits
	switch options.DataBits {
	case 5:
		result.c_cflag |= CS5
	case 6:
		result.c_cflag |= CS6
	case 7:
		result.c_cflag |= CS7
	case 8:
		result.c_cflag |= CS8
	default:
		return nil, os.NewError("Invalid setting for DataBits.")
	}

	return &result, nil
}

func openInternal(options OpenOptions) (io.ReadWriteCloser, os.Error) {
	// Open the serial port in non-blocking mode, since otherwise the OS will
	// wait for the CARRIER line to be asserted.
	file, err :=
		os.OpenFile(
			options.PortName,
			os.O_RDWR|os.O_NOCTTY|os.O_NONBLOCK,
			0600)

	if err != nil {
		return nil, err
	}

	// We want to do blocking I/O, so clear the non-blocking flag set above.
	r1, _, errno :=
		syscall.Syscall(
			syscall.SYS_FCNTL,
			uintptr(file.Fd()),
			uintptr(syscall.F_SETFL),
			uintptr(0))

	if err := os.NewSyscallError("SYS_IOCTL", int(errno)); err != nil {
		return nil, err
	}

	if r1 != 0 {
		return nil, os.NewError("Unknown error from SYS_FCNTL.")
	}

	// Set appropriate options.
	terminalOptions, err := convertOptions(options)
	if err != nil {
		return nil, err
	}

	err = setTermios(file.Fd(), terminalOptions)
	if err != nil {
		return nil, err
	}

	// We're done.
	return file, nil
}
