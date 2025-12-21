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

type MessageField struct {
	Name     string
	Type     FieldType
	Extra    any
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
					Extra:    nil,
					Optional: false,
				},
				{
					Name:     "currVersion",
					Type:     TypeInt32,
					Extra:    nil,
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
					Extra:    nil,
					Optional: false,
				},
				{
					Name:     "currVersion",
					Type:     TypeInt32,
					Extra:    nil,
					Optional: false,
				},
				{
					Name:     "schema",
					Type:     TypeLongBinary,
					Extra:    nil,
					Optional: false,
				},
				{
					Name:     "descriptorRegistry",
					Type:     TypeLongBinary,
					Extra:    nil,
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
					Extra:    nil,
					Optional: false,
				},
			},
		},
	},
}
