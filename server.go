package schemaipc

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/benjamin-larsen/goschemaipc/schema"
)

type MessageOverflowPolicy int

const (
  MessageOverflowDiscard MessageOverflowPolicy = iota
  MessageOverflowTerminate
)

type Server struct {
	Schema schema.Schema
	Listener net.Listener
	MessageOverflowPolicy MessageOverflowPolicy
	MaxMessageSize uint32
	Registry schema.MessageDescriptorRegistry
}

func (s *Server) Init() {
	if s.MessageOverflowPolicy != MessageOverflowDiscard && s.MessageOverflowPolicy != MessageOverflowTerminate {
		log.Fatal("Invalid Message Overflow Policy (must be Discard or Terminate)")
	}

	err := s.Registry.RegisterInternal()

	if err != nil {
		log.Fatal(err)
	}

	err = s.Registry.RegisterSchema(s.Schema)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) ListenAndServe(network, address string) error {
	listener, err := net.Listen(network, address)

	if err != nil {
		return err
	}

	s.Listener = listener

	defer listener.Close()

	log.Print("SchemaIPC Listening")

	for {
		conn, err := listener.Accept()

		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}

			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				log.Printf("Tempoary Error occured while Accepting Connection: %v", err)
				time.Sleep(3 * time.Second)
				continue
			}
			
			log.Printf("Permanent Error occured while Accepting Connection: %v", err)
			return err
		}

		go s.HandleConnection(conn)
	}

	log.Print("SchemaIPC Server Shutting Down...")

	return nil
}

func (s *Server) HandleConnection(netConn net.Conn) {
	log.Print("Socket Open")
	defer netConn.Close()

	var c = Conn{
		server: s,
		conn: netConn,
		state: ConnWaitHello,
	}

	for {
		err := c.NextMessage()

		if err != nil {
			break
		}
	}

	log.Print("Socket Close")
}