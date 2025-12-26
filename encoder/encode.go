package encoder

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"reflect"
	"slices"

	"github.com/benjamin-larsen/goschemaipc/schema"
)

var ErrRequiredNotPresent = errors.New("required field not present in struct")
var ErrWrongLen = errors.New("fixed binary field: wrong length")
var ErrLenTooBig16 = errors.New("binary field: length too long (must not be more than 65,535 bytes)")
var ErrLenTooBig32 = errors.New("binary field: length too long (must not be more than 4.29 GB)")
var ErrArrLenTooBig = errors.New("array field: length too long (must not be more than 65,535 elements)")

type Writer struct {
	buffer []byte
	pos    uint32
}

func (w *Writer) GrowBytes(n uint32) (uint32, error) {
	/*if n > utils.MaxInt {
		return 0, ErrOutOfBounds
	}*/

	currPos := w.pos
	newPos := currPos + n

	w.buffer = slices.Grow(w.buffer, int(n))[:newPos]
	w.pos = newPos

	return currPos, nil
}

func Encode(descriptor schema.MessageDescriptor, res any) (bytes []byte, err error) {
	writer := Writer{
		buffer: make([]byte, 0, descriptor.GetFixedSize()),
		pos:    0,
	}

	v := reflect.ValueOf(res)

	if v.Kind() != reflect.Struct {
		return nil, ErrInvalidResultPointer
	}

	defer func() {
		if r := recover(); r != nil {
			log.Println("Panic Occured in Encode", r)

			str, ok := r.(string)

			bytes = nil

			if ok {
				err = fmt.Errorf("panic: %s", str)
			} else {
				err = fmt.Errorf("panic: a panic occured")
			}
		}
	}()

	err = writer.encodeStruct(descriptor, v)

	if err != nil {
		return nil, err
	}

	return slices.Clip(writer.buffer), nil
}

func (w *Writer) SetOpt(opt uint32, offset uint32) {
	// same as rawBitPos % 8 but optimized
	bitPos := opt & 7

	// same as rawBitPos / 8 but optimized
	bytePos := offset + (opt >> 3)

	bitMask := byte(1 << bitPos)

	w.buffer[bytePos] |= bitMask
}

func (w *Writer) encodeStruct(descriptor schema.MessageDescriptor, v reflect.Value) error {
	if v.Kind() != reflect.Struct {
		return ErrInvalidResultPointer
	}

	t := v.Type()
	fMap, err := computeFieldMap(t)

	if err != nil {
		return err
	}

	// Start Decoding

	optBytes := descriptor.OptFlagLength()

	optListOffset, err := w.GrowBytes(optBytes)

	if err != nil {
		return err
	}

	var optCounter uint32 = 0

	for _, field := range descriptor.Message.Fields {
		fIdx, exists := fMap[field.Name]
		f := reflect.Value{}

		if exists {
			f = v.Field(fIdx)

			// Re-use "exists" variable for optional check
			exists = !f.IsZero()
		}

		if field.Optional {
			if optCounter >= optBytes {
				return ErrOptionalCorrupted
			}

			opt := optCounter
			optCounter++

			if exists {
				w.SetOpt(opt, optListOffset)
			} else {
				continue
			}
		}

		err := w.encodeSingle(field, f)

		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) encodeSingle(field schema.MessageField, f reflect.Value) error {
	// Future Note: use a default value instead of failing
	if !f.IsValid() {
		return ErrRequiredNotPresent
	}

	switch field.Type {
	case schema.TypeFixedBinary:
		{
			expectedLen := field.Extra.(int)
			bytes := f.Bytes()

			if len(bytes) != expectedLen {
				return ErrWrongLen
			}

			w.buffer = append(w.buffer, bytes...)

			break
		}
	case schema.TypeDynamicBinary:
		{
			bytes := f.Bytes()
			byteLen := len(bytes)

			if byteLen > 65535 {
				return ErrLenTooBig16
			}

			w.buffer = binary.LittleEndian.AppendUint16(w.buffer, uint16(byteLen))
			w.buffer = append(w.buffer, bytes...)

			break
		}
	case schema.TypeLongBinary:
		{
			bytes := f.Bytes()
			byteLen := len(bytes)

			if byteLen > 4294967295 {
				return ErrLenTooBig32
			}

			w.buffer = binary.LittleEndian.AppendUint32(w.buffer, uint32(byteLen))
			w.buffer = append(w.buffer, bytes...)

			break
		}
	case schema.TypeUInt64:
		{
			num := f.Uint()
			w.buffer = binary.LittleEndian.AppendUint64(w.buffer, num)

			break
		}
	case schema.TypeInt64:
		{
			num := f.Int()
			w.buffer = binary.LittleEndian.AppendUint64(w.buffer, uint64(num))

			break
		}
	case schema.TypeUInt32:
		{
			num := uint32(f.Uint())
			w.buffer = binary.LittleEndian.AppendUint32(w.buffer, num)

			break
		}
	case schema.TypeInt32:
		{
			num := int32(f.Int())
			w.buffer = binary.LittleEndian.AppendUint32(w.buffer, uint32(num))

			break
		}
	case schema.TypeUInt16:
		{
			num := uint16(f.Uint())
			w.buffer = binary.LittleEndian.AppendUint16(w.buffer, num)

			break
		}
	case schema.TypeInt16:
		{
			num := int16(f.Int())
			w.buffer = binary.LittleEndian.AppendUint16(w.buffer, uint16(num))

			break
		}
	case schema.TypeObject:
		{

			subFields := field.Extra.(schema.MessageDescriptor)

			err := w.encodeStruct(subFields, f)

			if err != nil {
				return err
			}

			break
		}
	case schema.TypeArray:
		{
			arrLen := f.Len()

			if arrLen > 65535 {
				return ErrArrLenTooBig
			}

			w.buffer = binary.LittleEndian.AppendUint16(w.buffer, uint16(arrLen))

			switch e := field.Extra.(type) {
			case schema.MessageField: {
				for i := 0; i < arrLen; i++ {
					item := f.Index(i)

					err := w.encodeSingle(e, item)

					if err != nil {
						return err
					}
				}
			}
			case schema.SchemaMessage: {
				desc := schema.MessageDescriptor{
					ID:            0,
					Message:       e,
					OptionalCount: e.CountOptional(),
					Internal:      false,
					Handler:       nil,
				}

				for i := 0; i < arrLen; i++ {
					item := f.Index(i)

					err := w.encodeStruct(desc, item)

					if err != nil {
						return err
					}
				}
			}
			}

			break
		}
	default:
		{
			return ErrTypeCorrupted
		}
	}

	return nil
}