package schemaipc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
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

func (c *Conn) NextMessage() error {
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

	payload, err := c.readPaylod(header.PacketLength)

	if err != nil {
		return err
	}

	fmt.Printf("Header: %+v\nPayload: %v\n", header, string(payload))

	return nil
}