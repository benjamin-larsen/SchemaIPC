package schema

import (
	"errors"
	"fmt"
	"strings"
)

type Conn interface{}

type Reader interface {
	// constraint: decode does not work concurrently
	Decode(res any) error
}

type HandlerFunc func(r Reader, c Conn) error

type MessageDescriptor struct {
	ID            uint32
	Message       SchemaMessage
	OptionalCount uint32
	Internal      bool
	OffRecord     bool
	Handler       HandlerFunc `json:"-"`
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

func (m MessageDescriptor) GetFixedSize() uint32 {
	var accum uint32 = m.OptFlagLength()

	for _, field := range m.Message.Fields {
		accum += field.Type.GetFixedSize(field.Extra)
	}

	return accum
}

type SignatureMap map[string]uint32

type MessageDescriptorRegistry struct {
	idCounter            uint32
	RegisteredUser       bool
	RegisteredInternal   bool
	Descriptors          map[uint32]MessageDescriptor
	UserSignatureMap     SignatureMap // Maps User-defined Message Signature to Message Descriptor ID
	InternalSignatureMap SignatureMap // Maps Internal Message Signature to Message Descriptor ID
}

var ErrAlreadyRegistered = errors.New("schema is already registered")
var ErrInternalNotRegistered = errors.New("internal schema is not registered")

func (r *MessageDescriptorRegistry) ensureDescriptors() {
	if r.Descriptors == nil {
		r.Descriptors = make(map[uint32]MessageDescriptor)
		r.UserSignatureMap = make(SignatureMap)
		r.InternalSignatureMap = make(SignatureMap)
	}
}

func registerSignature(signatureMap SignatureMap, direction MessageDirection, name string, id uint32) error {
	signature := fmt.Sprintf("%s %s", direction.ToString(), name)

	_, exists := signatureMap[signature]

	if exists {
		return fmt.Errorf("duplicate signature: %s", signature)
	}

	signatureMap[signature] = id

	return nil
}

func handleSignatures(signatureMap SignatureMap, message SchemaMessage, id uint32) error {
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

func (r *MessageDescriptorRegistry) resolveMessageField(field *MessageField, internal bool) {
	if field.Type == TypeObject {
		signature, ok := field.Extra.(string)

		if !ok {
			// already been resolved
			return
		}

		if !strings.HasPrefix(signature, "object ") {
			panic("object field must specify a object signature")
		}

		var signatureMap SignatureMap

		if internal {
			signatureMap = r.InternalSignatureMap
		} else {
			signatureMap = r.UserSignatureMap
		}

		descriptorID, exists := signatureMap[signature]

		if !exists {
			panic("object signature doesn't exist")
		}

		descriptor, exists := r.Descriptors[descriptorID]

		if !exists {
			panic("internal error: descriptor not found")
		}

		// field.Type == TypeArray
		r.resolveMessageFields(&descriptor.Message, internal)

		field.Extra = descriptor
	} else if field.Type == TypeArray {
		subField, ok := field.Extra.(MessageField)

		if !ok {
			// already been resolved
			return
		}

		r.resolveMessageField(&subField, internal)

		field.Extra = subField
	}
}

func (r *MessageDescriptorRegistry) resolveMessageFields(message *SchemaMessage, internal bool) {
	for idx, field := range message.Fields {
		r.resolveMessageField(&field, internal)
		message.Fields[idx] = field
	}
}

func (r *MessageDescriptorRegistry) ResolveMessages() {
	for _, descriptor := range r.Descriptors {
		r.resolveMessageFields(&descriptor.Message, descriptor.Internal)
	}
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

func (r *MessageDescriptorRegistry) RegisterOffRecord() error {
	if r.RegisteredInternal || r.RegisteredUser {
		return ErrAlreadyRegistered
	}

	r.ensureDescriptors()

	for _, message := range InternalOffrecord.Messages {
		id := r.idCounter

		r.idCounter++

		r.Descriptors[id] = MessageDescriptor{
			ID:            id,
			Message:       message,
			OptionalCount: message.CountOptional(),
			Internal:      true,
			OffRecord:     true,
			Handler:       nil,
		}

		err := handleSignatures(r.InternalSignatureMap, message, id)

		if err != nil {
			return err
		}
	}

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
