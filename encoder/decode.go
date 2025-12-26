package encoder

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/benjamin-larsen/goschemaipc/schema"
)

var ErrOutOfBounds = errors.New("out of bounds")
var ErrOptionalCorrupted = errors.New("optional count is corrupted")
var ErrTypeCorrupted = errors.New("message type is corrupted")
var ErrInvalidResultObject = errors.New("invalid result object (expected *struct)")
var ErrInvalidResultPointer = errors.New("invalid result poinetr (expected struct)")
var ErrInvalidByteKind = errors.New("invalid field kind (expected Array ([N]byte), Slice ([]byte) or string)")

type Reader struct {
	buffer       []byte
	descriptor   schema.MessageDescriptor
	pos          uint32
	len          uint32
}

func NewReader(buffer []byte, descriptor schema.MessageDescriptor) Reader {
	return Reader{
		buffer:       buffer,
		descriptor: descriptor,
		pos:          0,
		len: uint32(len(buffer)),
	}
}

func (r *Reader) ReadBytes(n uint32) ([]byte, error) {
	if n > (r.len - r.pos) {
		return nil, ErrOutOfBounds
	}

	result := r.buffer[r.pos : r.pos+n]

	if uint32(len(result)) != n {
		return nil, ErrOutOfBounds
	}

	r.pos += n

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

func (r *Reader) Decode(res any) error {
	r.pos = 0

	// Setup reflection

	vPtr := reflect.ValueOf(res)

	if vPtr.Kind() != reflect.Ptr {
		return ErrInvalidResultObject
	}

	return r.decodeStruct(r.descriptor, vPtr.Elem())
}

func (r *Reader) decodeStruct(descriptor schema.MessageDescriptor, v reflect.Value) error {
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

		fIdx, exists := fMap[field.Name]
		f := reflect.Value{}

		if exists {
			f = v.Field(fIdx)
		}

		err := r.decodeSingle(field, f)

		if err != nil {
			return err
		}
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

func setBytes(bytes []byte, v reflect.Value) error {
	addr := unsafe.Pointer(v.UnsafeAddr())
	kind := v.Kind()

	switch kind {
	case reflect.Array:
		{
			byteLen := len(bytes)
			arrLen := v.Len()

			if byteLen != arrLen {
				return ErrWrongLen
			}

			copy(
				unsafe.Slice((*byte)(addr), arrLen),
				bytes,
			)
			break
		}
	case reflect.Slice:
		{
			// check that elem is byte too
			*(*[]byte)(addr) = bytes
			break
		}
	case reflect.String:
		{
			*(*string)(addr) = string(bytes)
			break
		}
	default:
		{
			return ErrInvalidByteKind
		}
	}

	return nil
}

// TODO: check types
// Check IsValid and CanSet
func (r *Reader) decodeSingle(field schema.MessageField, f reflect.Value) error {
	switch field.Type {
	case schema.TypeFixedBinary:
		{
			fLen := field.Extra.(int)
			bytes, err := r.ReadBytes(uint32(fLen))

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			err = setBytes(bytes, f)

			if err != nil {
				return err
			}

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

			if !f.IsValid() {
				return nil
			}

			err = setBytes(bytes, f)

			if err != nil {
				return err
			}

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

			if !f.IsValid() {
				return nil
			}

			err = setBytes(bytes, f)

			if err != nil {
				return err
			}

			break
		}
	case schema.TypeUInt64:
		{
			num, err := r.ReadUInt64()

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			f.SetUint(num)

			break
		}
	case schema.TypeInt64:
		{
			num, err := r.ReadInt64()

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			f.SetInt(num)

			break
		}
	case schema.TypeUInt32:
		{
			num, err := r.ReadUInt32()

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			f.SetUint(uint64(num))

			break
		}
	case schema.TypeInt32:
		{
			num, err := r.ReadInt32()

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			f.SetInt(int64(num))

			break
		}
	case schema.TypeUInt16:
		{
			num, err := r.ReadUInt16()

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			f.SetUint(uint64(num))

			break
		}
	case schema.TypeInt16:
		{
			num, err := r.ReadInt16()

			if err != nil {
				return err
			}

			if !f.IsValid() {
				return nil
			}

			f.SetInt(int64(num))

			break
		}
	case schema.TypeObject:
		{
			subFields := field.Extra.(schema.MessageDescriptor)

			if !f.IsValid() {
				// need to skip bytes here
				return nil
			}

			err := r.decodeStruct(subFields, f)

			if err != nil {
				return err
			}

			break
		}
	case schema.TypeArray:
		{
			lenU32, err := r.ReadUInt16()

			if err != nil {
				return err
			}

			arrLen := int(lenU32)

			if !f.IsValid() {
				// need to skip bytes here
				return nil
			}

			switch e := field.Extra.(type) {
			case schema.MessageField: {
				itemSize := e.Type.GetFixedSize(e.Extra) * uint32(arrLen)

				if itemSize > (r.len - r.pos) {
					return ErrOutOfBounds
				}

				slice := reflect.MakeSlice(f.Type(), arrLen, arrLen)

				f.Set(slice)

				for i := 0; i < arrLen; i++ {
					item := slice.Index(i)

					err := r.decodeSingle(e, item)

					if err != nil {
						return err
					}
				}
			}
			case schema.MessageDescriptor: {
				itemSize := e.GetFixedSize() * uint32(arrLen)

				if itemSize > (r.len - r.pos) {
					return ErrOutOfBounds
				}

				slice := reflect.MakeSlice(f.Type(), arrLen, arrLen)

				f.Set(slice)

				for i := 0; i < arrLen; i++ {
					item := slice.Index(i)

					err := r.decodeStruct(e, item)

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
