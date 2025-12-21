package encoder

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"

	"github.com/benjamin-larsen/goschemaipc/schema"
)

var ErrOutOfBounds = errors.New("out of bounds")
var ErrOptionalCorrupted = errors.New("optional count is corrupted")
var ErrTypeCorrupted = errors.New("message type is corrupted")
var ErrInvalidResultObject = errors.New("invalid result object (expected *Struct)")

type Reader struct {
	buffer       []byte
	pos          uint32
	availableLen uint32
}

func NewReader(buffer []byte) Reader {
	return Reader{
		buffer:       buffer,
		pos:          0,
		availableLen: uint32(len(buffer)),
	}
}

func (r *Reader) ReadBytes(n uint32) ([]byte, error) {
	if n > r.availableLen {
		return nil, ErrOutOfBounds
	}

	result := r.buffer[r.pos : r.pos+n]

	if uint32(len(result)) != n {
		return nil, ErrOutOfBounds
	}

	r.pos += n
	r.availableLen -= n

	return result, nil
}

func (r *Reader) ReadUInt16() (uint16, error) {
	bytes, err := r.ReadBytes(2)

	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint16(bytes), nil
}

func (r *Reader) ReadInt16() (int16, error) {
	bytes, err := r.ReadBytes(2)

	if err != nil {
		return 0, err
	}

	return int16(binary.LittleEndian.Uint16(bytes)), nil
}

func (r *Reader) ReadUInt32() (uint32, error) {
	bytes, err := r.ReadBytes(4)

	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(bytes), nil
}

func (r *Reader) ReadInt32() (int32, error) {
	bytes, err := r.ReadBytes(4)

	if err != nil {
		return 0, err
	}

	return int32(binary.LittleEndian.Uint32(bytes)), nil
}

func (r *Reader) ReadUInt64() (uint64, error) {
	bytes, err := r.ReadBytes(8)

	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint64(bytes), nil
}

func (r *Reader) ReadInt64() (int64, error) {
	bytes, err := r.ReadBytes(8)

	if err != nil {
		return 0, err
	}

	return int64(binary.LittleEndian.Uint64(bytes)), nil
}

func GetOpt(opt uint32, optList []byte) bool {
	// same as rawBitPos % 8 but optimized
	bitPos := opt & 7

	// same as rawBitPos / 8 but optimized
	bytePos := opt >> 3

	bitMask := byte(1 << bitPos)

	return (optList[bytePos] & bitMask) != 0
}

type fieldMap map[string]int // field name (protocol) to field index (struct)

var typeCache sync.Map // map[reflect.Type]fieldMap

func computeFieldMap(t reflect.Type) (fieldMap, error) {
	cached, exists := typeCache.Load(t)

	if exists {
		return cached.(fieldMap), nil
	}

	numField := t.NumField()
	fMap := make(fieldMap, numField)

	for i := 0; i < numField; i++ {
		field := t.Field(i)
		tag := field.Tag.Get("ipc")

		if tag == "" {
			continue
		}

		_, exists := fMap[tag]

		if exists {
			return nil, fmt.Errorf("duplicate struct tag: %s", tag)
		}

		fMap[tag] = i
	}

	cached, _ = typeCache.LoadOrStore(t, fMap)

	return cached.(fieldMap), nil
}

func (r *Reader) Decode(descriptor schema.MessageDescriptor, res any) error {
	// Setup reflection

	vPtr := reflect.ValueOf(res)
	v := vPtr.Elem()

	if vPtr.Kind() != reflect.Ptr || v.Kind() != reflect.Struct {
		return ErrInvalidResultObject
	}

	t := v.Type()
	fMap, err := computeFieldMap(t)

	if err != nil {
		return err
	}

	// Start Decoding

	optBytes := descriptor.OptFlagLength()

	optList, err := r.ReadBytes(optBytes)

	if err != nil {
		return err
	}

	var optCounter uint32 = 0

	for _, field := range descriptor.Message.Fields {
		if field.Optional {
			if optCounter >= optBytes {
				return ErrOptionalCorrupted
			}

			opt := optCounter
			optCounter++

			if !GetOpt(opt, optList) {
				continue
			}
		}

		r.decodeSingle(field, fMap, v)

		log.Printf("Field: %+v\n", field)
	}

	return nil
}

/*
		TypeFixedBinary
TypeDynamicBinary
TypeLongBinary
TypeUInt64
TypeInt64
TypeUInt32
TypeInt32
TypeUInt16
TypeInt16
*/

// TODO: check types
// Check IsValid and CanSet
func (r *Reader) decodeSingle(field schema.MessageField, fMap fieldMap, v reflect.Value) error {
	fIdx, exists := fMap[field.Name]

	switch field.Type {
	case schema.TypeFixedBinary:
		{
			bytes, err := r.ReadBytes(field.Extra.(uint32))

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetBytes(bytes)

			break
		}
	case schema.TypeDynamicBinary:
		{
			len, err := r.ReadUInt16()

			if err != nil {
				return err
			}

			bytes, err := r.ReadBytes(uint32(len))

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetBytes(bytes)

			break
		}
	case schema.TypeLongBinary:
		{
			len, err := r.ReadUInt32()

			if err != nil {
				return err
			}

			bytes, err := r.ReadBytes(uint32(len))

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetBytes(bytes)

			break
		}
	case schema.TypeUInt64:
		{
			num, err := r.ReadUInt64()

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetUint(num)

			break
		}
	case schema.TypeInt64:
		{
			num, err := r.ReadInt64()

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetInt(num)

			break
		}
	case schema.TypeUInt32:
		{
			num, err := r.ReadUInt32()

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetUint(uint64(num))

			break
		}
	case schema.TypeInt32:
		{
			num, err := r.ReadInt32()

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetInt(int64(num))

			break
		}
	case schema.TypeUInt16:
		{
			num, err := r.ReadUInt16()

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetUint(uint64(num))

			break
		}
	case schema.TypeInt16:
		{
			num, err := r.ReadInt16()

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			f := v.Field(fIdx)
			f.SetInt(int64(num))

			break
		}
	default:
		{
			return ErrTypeCorrupted
		}
	}

	return nil
}
