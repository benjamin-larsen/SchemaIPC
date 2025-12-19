package schemaipc

import "errors"

type MessageOverflowPolicy int

const (
  MessageOverflowDiscard MessageOverflowPolicy = iota
  MessageOverflowTerminate
)

type ConnState int

const (
	ConnWaitHello = iota
	ConnEstablished
)

var ErrHeaderLength = errors.New("invalid header length (must be 8 bytes)")
var ErrMsgLength = errors.New("exceeded message limit")

var ErrMsgReadLength = errors.New("invalid payload length")