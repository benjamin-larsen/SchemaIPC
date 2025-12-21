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

type fieldMapEntry struct {
	Offset uintptr
	Kind reflect.Kind
	ElemKind reflect.Kind
	ElemCount int // for Array
	IsPointer bool
}

type fieldMap map[string]fieldMapEntry // field name (protocol) to field offset (struct)

var typeCache sync.Map // map[reflect.Type]fieldMap2

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

		elemKind := reflect.Invalid
		elemCount := 0
		ft := field.Type
		isPointer := false

		// unwrap pointer
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
			isPointer = true
		}

		ftKind := ft.Kind()

		// set element kind (for slice or array / fixed slices)
		if ftKind == reflect.Slice || ftKind == reflect.Array {
			elemKind = ft.Elem().Kind()
		}
		
		// set count for fixed slices
		if ftKind == reflect.Array {
			elemCount = ft.Len()
		}

		fMap[tag] = fieldMapEntry{
			Offset: field.Offset,
			Kind: ft.Kind(),
			ElemKind: elemKind,
			ElemCount: elemCount,
			IsPointer: isPointer,
		}
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

	basePtr := v.UnsafeAddr()

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

		r.decodeSingle(field, fMap, basePtr)
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

/*
allow pointers to these to be used as "optional", remember to use unsafe.Pointer() and not uintptr to ensure the Garbage Man doesn't pick up living data

TypeFixedBinary, TypeDynamicBinary and TypeLongBinary can be Array (must be exact size) or Slice
*/

// TODO: check types
// Check IsValid and CanSet
func (r *Reader) decodeSingle(field schema.MessageField, fMap fieldMap, basePtr uintptr) error {
	fEntry, exists := fMap[field.Name]

	switch field.Type {
	case schema.TypeFixedBinary:
		{
			len := field.Extra.(int)
			bytes, err := r.ReadBytes(uint32(len))

			if err != nil {
				return err
			}

			if !exists {
				return nil
			}

			// do checks before, and if pointer need to make a new heap allocation
			*(*[]byte)(unsafe.Pointer(basePtr + fEntry.Offset)) = bytes

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*[]byte)(unsafe.Pointer(basePtr + fEntry.Offset)) = bytes

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*[]byte)(unsafe.Pointer(basePtr + fEntry.Offset)) = bytes

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*uint64)(unsafe.Pointer(basePtr + fEntry.Offset)) = num

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*int64)(unsafe.Pointer(basePtr + fEntry.Offset)) = num

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*uint32)(unsafe.Pointer(basePtr + fEntry.Offset)) = num

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*int32)(unsafe.Pointer(basePtr + fEntry.Offset)) = num

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*uint16)(unsafe.Pointer(basePtr + fEntry.Offset)) = num

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

			// do checks before, and if pointer need to make a new heap allocation
			*(*int16)(unsafe.Pointer(basePtr + fEntry.Offset)) = num

			break
		}
	default:
		{
			return ErrTypeCorrupted
		}
	}

	return nil
}
