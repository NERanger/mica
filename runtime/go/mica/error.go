package mica

import "fmt"

type RpcCode int

const (
	RpcCodeUnspecified     RpcCode = 0
	RpcCodeOK              RpcCode = 1
	RpcCodeInvalidArgument RpcCode = 2
	RpcCodeNotFound        RpcCode = 3
	RpcCodeUnavailable     RpcCode = 4
	RpcCodeInternal        RpcCode = 5
)

func (c RpcCode) String() string {
	switch c {
	case RpcCodeOK:
		return "OK"
	case RpcCodeInvalidArgument:
		return "INVALID_ARGUMENT"
	case RpcCodeNotFound:
		return "NOT_FOUND"
	case RpcCodeUnavailable:
		return "UNAVAILABLE"
	case RpcCodeInternal:
		return "INTERNAL"
	default:
		return "UNSPECIFIED"
	}
}

func isWireError(code RpcCode) bool {
	switch code {
	case RpcCodeInvalidArgument, RpcCodeNotFound, RpcCodeUnavailable, RpcCodeInternal:
		return true
	default:
		return false
	}
}

type RpcError struct {
	Code    RpcCode
	Message string
}

func (e *RpcError) Error() string {
	if e.Message == "" {
		return e.Code.String()
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewRpcError(code RpcCode, message string) *RpcError {
	return &RpcError{Code: code, Message: message}
}

type CallTimeout struct {
	Message string
	Err     error
}

func (e *CallTimeout) Error() string {
	if e.Message == "" {
		return "rpc timed out"
	}
	return e.Message
}

func (e *CallTimeout) Unwrap() error { return e.Err }

func NewCallTimeout(message string, err error) *CallTimeout {
	return &CallTimeout{Message: message, Err: err}
}

type CallCancelled struct {
	Message string
	Err     error
}

func (e *CallCancelled) Error() string {
	if e.Message == "" {
		return "rpc cancelled"
	}
	return e.Message
}

func (e *CallCancelled) Unwrap() error { return e.Err }

func NewCallCancelled(message string, err error) *CallCancelled {
	return &CallCancelled{Message: message, Err: err}
}

type TransportError struct {
	Message string
	Err     error
}

func (e *TransportError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *TransportError) Unwrap() error { return e.Err }

type ProtocolError struct {
	Message string
}

func (e *ProtocolError) Error() string { return e.Message }
