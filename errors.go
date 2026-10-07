package mikorm

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned by Query.First when no record matches.
	ErrNotFound = errors.New("mikorm: record not found")

	// ErrClosed is returned when using a Client after Close was called.
	ErrClosed = errors.New("mikorm: client is closed")
)

// DeviceError is an error reported by the RouterOS device (a !trap or
// !fatal reply).
type DeviceError struct {
	// Command is the API command that failed, e.g. "/ip/address/add".
	Command string
	// Message is the error message sent by the device.
	Message string
	// Category is the RouterOS error category, if the device sent one.
	Category string
	// Fatal reports whether the device closed the connection (!fatal).
	Fatal bool
}

func (e *DeviceError) Error() string {
	return fmt.Sprintf("mikorm: %s: %s", e.Command, e.Message)
}
