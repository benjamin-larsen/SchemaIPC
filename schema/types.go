package schema

type FieldType int

const (
	TypeFixedBinary FieldType = iota
	TypeDynamicBinary
	TypeLongBinary
	TypeUInt64
	TypeInt64
	TypeUInt32
	TypeInt32
	TypeUInt16
	TypeInt16
	TypeObject
	TypeArray
)

func (f FieldType) GetFixedSize(extra any) uint32 {
	switch f {
	case TypeFixedBinary: // binary(N): return N
		len := extra.(int)
		return uint32(len)

	case TypeDynamicBinary: // binary return 2 for the length-prefix
		return 2

	case TypeLongBinary: // long_binary: return 4 for the length-prefix
		return 4

	case TypeUInt64:
	case TypeInt64:
		return 8

	case TypeUInt32:
	case TypeInt32:
		return 4
		
	case TypeUInt16:
	case TypeInt16:
		return 2
	
	case TypeObject:
		return extra.(MessageDescriptor).GetFixedSize()
	
	case TypeArray: // array: return 2 for the length-prefix
		return 2
	}

	return 0
}
