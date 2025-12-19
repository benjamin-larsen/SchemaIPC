package schema

type MessageDirection int

const (
	InboundMessage MessageDirection = iota
	OutboundMessage
	DuplexMessage
)

func (d MessageDirection) ToString() string {
	switch d {
	case InboundMessage:
		return "inbound"
	case OutboundMessage:
		return "outbound"
	case DuplexMessage:
		return "duplex"
	default:
		return ""
	}
}

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

type MessageField struct {
	Name     string
	Type     FieldType
	Length   uint32
	Optional bool
}

type SchemaMessage struct {
	Direction MessageDirection
	Name      string
	Fields    []MessageField
}

func (m SchemaMessage) CountOptional() uint32 {
	var i uint32 = 0

	for _, field := range m.Fields {
		if field.Optional {
			i++
		}
	}

	return i
}

type Schema struct {
	Messages []SchemaMessage
}

// Inbound and Outbound Hello must both be ID 0 and 1 respectively, never change this
// Exclude first 2 (Inbound and Outbound Hello) from the Descriptor Registry over wire
var InternalSchema = Schema{
	Messages: []SchemaMessage{
		{
			Direction: InboundMessage,
			Name:      "Hello",
			Fields: []MessageField{
				{
					Name:     "minVersion",
					Type:     TypeInt32,
					Length:   0,
					Optional: false,
				},
				{
					Name:     "currVersion",
					Type:     TypeInt32,
					Length:   0,
					Optional: false,
				},
			},
		},

		{
			Direction: OutboundMessage,
			Name:      "Hello",
			Fields: []MessageField{
				{
					Name:     "minVersion",
					Type:     TypeInt32,
					Length:   0,
					Optional: false,
				},
				{
					Name:     "currVersion",
					Type:     TypeInt32,
					Length:   0,
					Optional: false,
				},
				{
					Name:     "schema",
					Type:     TypeLongBinary,
					Length:   0,
					Optional: false,
				},
				{
					Name:     "descriptorRegistry",
					Type:     TypeLongBinary,
					Length:   0,
					Optional: false,
				},
			},
		},

		{
			Direction: DuplexMessage,
			Name:      "Ping",
			Fields: []MessageField{
				{
					Name:     "timestamp",
					Type:     TypeInt64,
					Length:   0,
					Optional: false,
				},
			},
		},
	},
}
