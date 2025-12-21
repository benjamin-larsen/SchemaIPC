package schemaipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/benjamin-larsen/goschemaipc/encoder"
	"github.com/benjamin-larsen/goschemaipc/schema"
)

var ErrHeaderLength = errors.New("invalid header length (must be 8 bytes)")
var ErrMsgLength = errors.New("exceeded message limit")

var ErrMsgReadLength = errors.New("invalid payload length")

var ErrSentInvalidDirection = errors.New("client attempted to send a outbound message")
var ErrInvalidDescriptor = errors.New("client attempted to send a unknown message")

type ConnState int

const (
	ConnWaitHello = iota
	ConnEstablished
)

type ProtocolHeader struct {
	PacketLength uint32
	MessageType uint32
}

type Conn struct {
	server *Server
	conn net.Conn
	state ConnState
}

var nullHeader = ProtocolHeader{}

func (c *Conn) readHeader() (ProtocolHeader, error) {
	var rawHeader [8]byte

	bytesRead, err := io.ReadFull(c.conn, rawHeader[:])

	if err != nil {
		return nullHeader, err
	}

	if bytesRead != 8 {
		return nullHeader, ErrHeaderLength
	}

	return ProtocolHeader{
		PacketLength: binary.LittleEndian.Uint32(rawHeader[:4]),
		MessageType: binary.LittleEndian.Uint32(rawHeader[4:]),
	}, nil
}

func (c *Conn) readPaylod(len uint32) ([]byte, error) {
	payload := make([]byte, len)

	bytesRead, err := io.ReadFull(c.conn, payload)

	if err != nil {
		return nil, err
	}

	if bytesRead != int(len) {
		return nil, ErrMsgReadLength
	}

	return payload, nil
}

func (c *Conn) nextMessage() error {
	header, err := c.readHeader()

	if err != nil {
		return err
	}

	if header.PacketLength > c.server.MaxMessageSize {
		switch (c.server.MessageOverflowPolicy) {
		case MessageOverflowDiscard:
			io.CopyN(io.Discard, c.conn, int64(header.PacketLength))
			return nil
		case MessageOverflowTerminate:
			return ErrMsgLength
		}
	}

	descriptor, exists := c.server.Registry.Descriptors[header.MessageType]

	if !exists {
		// Treat this as protocol violation, a proper client should never attempt to encode a Message not present on the Schema-on-Wire
		fmt.Printf("Unknown Message: %v\n", header.MessageType)
		return ErrInvalidDescriptor
	}

	if descriptor.Message.Direction == schema.OutboundMessage {
		// Treat this as protocol violation, a proper client should never attempt to encode a Outbound Message
		fmt.Printf("Invalid Message Direction for %v\n", header.MessageType)
		return ErrSentInvalidDirection
	}

	if !descriptor.Internal && c.state == ConnWaitHello {
		// Treat this as protocol violation, a proper client should never send a User-specified message before Connection is Established
		fmt.Printf("Sent user-specified message outside of Established Connection: %v\n", header.MessageType)
		return ErrInvalidDescriptor
	}

	if descriptor.Handler == nil {
		// Ignore User-defined Schemas that don't have handler
		io.CopyN(io.Discard, c.conn, int64(header.PacketLength))
		return nil
	}

	payload, err := c.readPaylod(header.PacketLength)

	if err != nil {
		return err
	}

	reader := encoder.NewReader(payload, descriptor)

	err = descriptor.Handler(&reader, c)

	if err != nil {
		return err
	}

	return nil
}