package schema

type MessageDirection int

const (
	InboundMessage MessageDirection = iota
	OutboundMessage
	DuplexMessage
	ObjectDef
)

func (d MessageDirection) ToString() string {
	switch d {
	case InboundMessage:
		return "inbound"
	case OutboundMessage:
		return "outbound"
	case DuplexMessage:
		return "duplex"
	case ObjectDef:
		return "object"
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

// Need a more elegant solution in future to make this mantainable

var v1WireSchema = MessageDescriptor{
	Internal: true,
	Message: SchemaMessage{
		Direction: ObjectDef,
		Name:      "schema",
		Fields: []MessageField{
			{
				Name: "descriptors",
				Type: TypeArray,
				Extra: MessageField{
					Type: TypeObject,
					Extra: MessageDescriptor{
						Internal: true,
						Message: SchemaMessage{
							Direction: ObjectDef,
							Name:      "messageDescriptor",
							Fields: []MessageField{
								{
									Name:     "id",
									Type:     TypeUInt32,
									Extra:    nil,
									Optional: false,
								},
								{
									Name:     "internal",
									Type:     TypeUInt16,
									Extra:    nil,
									Optional: false,
								},
								{
									Name:     "direction",
									Type:     TypeUInt16,
									Extra:    nil,
									Optional: false,
								},
								{
									Name:     "name",
									Type:     TypeDynamicBinary,
									Extra:    nil,
									Optional: false,
								},
								{
									Name: "fields",
									Type: TypeArray,
									Extra: MessageField{
										Type: TypeObject,
										Extra: MessageDescriptor{
											Internal: true,
											Message: SchemaMessage{
												Direction: ObjectDef,
												Name:      "messageField",
												Fields: []MessageField{
													{
														Name:     "name",
														Type:     TypeDynamicBinary,
														Extra:    nil,
														Optional: false,
													},
													{
														Name:     "type",
														Type:     TypeUInt16,
														Extra:    nil,
														Optional: false,
													},
													{
														Name:     "extra",
														Type:     TypeLongBinary,
														Extra:    nil,
														Optional: false,
													},
													{
														Name:     "optional",
														Type:     TypeUInt16,
														Extra:    nil,
														Optional: false,
													},
												},
											},
										},
									},
									Optional: false,
								},
							},
						},
					},
				},
				Optional: false,
			},
		},
	}}

var InternalOffrecord = Schema{
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
			},
		},
	},
}

var InternalSchema = Schema{
	Messages: []SchemaMessage{
		{
			Direction: OutboundMessage,
			Name:      "ProtocolError",
			Fields: []MessageField{
				{
					Name:     "message",
					Type:     TypeDynamicBinary,
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
