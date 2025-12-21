package schema

import (
	"errors"
	"fmt"
)

type MessageDescriptor struct {
	ID            uint32
	Message       SchemaMessage
	OptionalCount uint32
	Internal      bool
	Handler       interface{} `json:"-"`
}

func (m MessageDescriptor) OptFlagLength() uint32 {
	// divide by 8
	count := m.OptionalCount >> 3

	// Math.ceil-like
	if (m.OptionalCount & 7) != 0 {
		count++
	}

	return count
}

type MessageDescriptorRegistry struct {
	idCounter            uint32
	RegisteredUser       bool
	RegisteredInternal   bool
	Descriptors          map[uint32]MessageDescriptor
	UserSignatureMap     map[string]uint32 // Maps User-defined Message Signature to Message Descriptor ID
	InternalSignatureMap map[string]uint32 // Maps Internal Message Signature to Message Descriptor ID
}

var ErrAlreadyRegistered = errors.New("schema is already registered")
var ErrInternalNotRegistered = errors.New("internal schema is not registered")

func (r *MessageDescriptorRegistry) ensureDescriptors() {
	if r.Descriptors == nil {
		r.Descriptors = make(map[uint32]MessageDescriptor)
		r.UserSignatureMap = make(map[string]uint32)
		r.InternalSignatureMap = make(map[string]uint32)
	}
}

func registerSignature(signatureMap map[string]uint32, direction MessageDirection, name string, id uint32) error {
	signature := fmt.Sprintf("%s %s", direction.ToString(), name)

	_, exists := signatureMap[signature]

	if exists {
		return fmt.Errorf("duplicate signature: %s", signature)
	}

	signatureMap[signature] = id

	return nil
}

func handleSignatures(signatureMap map[string]uint32, message SchemaMessage, id uint32) error {
	if message.Direction == DuplexMessage {
		err := registerSignature(signatureMap, InboundMessage, message.Name, id)

		if err != nil {
			return err
		}

		err = registerSignature(signatureMap, OutboundMessage, message.Name, id)

		if err != nil {
			return err
		}
	} else {
		err := registerSignature(signatureMap, message.Direction, message.Name, id)

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *MessageDescriptorRegistry) RegisterSchema(schema Schema) error {
	if r.RegisteredUser {
		return ErrAlreadyRegistered
	}

	if !r.RegisteredInternal {
		return ErrInternalNotRegistered
	}

	r.ensureDescriptors()

	for _, message := range schema.Messages {
		id := r.idCounter
		r.idCounter++

		r.Descriptors[id] = MessageDescriptor{
			ID:            id,
			Message:       message,
			OptionalCount: message.CountOptional(),
			Internal:      false,
			Handler:       nil,
		}

		err := handleSignatures(r.UserSignatureMap, message, id)

		if err != nil {
			return err
		}
	}

	r.RegisteredUser = true

	return nil
}

func (r *MessageDescriptorRegistry) RegisterInternal() error {
	if r.RegisteredInternal || r.RegisteredUser {
		return ErrAlreadyRegistered
	}

	r.ensureDescriptors()

	for _, message := range InternalSchema.Messages {
		id := r.idCounter
		r.idCounter++

		r.Descriptors[id] = MessageDescriptor{
			ID:            id,
			Message:       message,
			OptionalCount: message.CountOptional(),
			Internal:      true,
			Handler:       nil,
		}

		err := handleSignatures(r.InternalSignatureMap, message, id)

		if err != nil {
			return err
		}
	}

	r.RegisteredInternal = true

	return nil
}
