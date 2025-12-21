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
)

