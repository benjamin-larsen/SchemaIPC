package encoder

import (
	"fmt"
	"log"
	"slices"
	"testing"

	exp "github.com/benjamin-larsen/goschemaipc/exp/encoder"
	"github.com/benjamin-larsen/goschemaipc/schema"
)

var sampleMsg = schema.SchemaMessage{
	Direction: schema.InboundMessage,
	Name:      "sample",
	Fields: []schema.MessageField{
		{
			Name:     "fixed",
			Type:     schema.TypeFixedBinary,
			Extra:    6,
			Optional: false,
		},
		{
			Name:     "dynamic",
			Type:     schema.TypeDynamicBinary,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "long",
			Type:     schema.TypeLongBinary,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "uint64",
			Type:     schema.TypeUInt64,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "int64",
			Type:     schema.TypeInt64,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "uint32",
			Type:     schema.TypeUInt32,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "int32",
			Type:     schema.TypeInt32,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "uint16",
			Type:     schema.TypeUInt16,
			Extra:    nil,
			Optional: false,
		},
		{
			Name:     "int16",
			Type:     schema.TypeInt16,
			Extra:    nil,
			Optional: false,
		},
		{
			Name: "array",
			Type: schema.TypeArray,
			Extra: schema.MessageField{
				Type: schema.TypeInt16,
			},
			Optional: false,
		},
	},
}

// place randomly to ensure no buffer overflow side-affect from raw pointer
type sampleStruct struct {
	Long    []byte `ipc:"long"`
	UInt64  uint64 `ipc:"uint64"`
	Int64   int64  `ipc:"int64"`
	Dynamic []byte `ipc:"dynamic"`
	UInt32  uint32 `ipc:"uint32"`
	Fixed   []byte `ipc:"fixed"`
	Int32   int32  `ipc:"int32"`
	UInt16  uint16 `ipc:"uint16"`
	Int16   int16  `ipc:"int16"`
	Array   []int16 `ipc:"array"`
}

type messageField struct {
	Name     []byte           `ipc:"name"`
	Type     uint16           `ipc:"type"`
	Extra    []byte           `ipc:"extra"`
	Optional uint16           `ipc:"optional"`
}

type messageDescriptor struct {
	ID        uint32         `ipc:"id"`
	Internal  uint16         `ipc:"internal"`
	Direction uint16         `ipc:"direction"`
	Name      []byte         `ipc:"name"`
	Fields    []messageField `ipc:"fields"`
}

type helloOutbound struct {
	MinVersion int32               `ipc:"minVersion"`
	Version    int32               `ipc:"currVersion"`
	Schema     []messageDescriptor `ipc:"schema"`
}

var sampleDesc = schema.MessageDescriptor{
	ID:            0,
	Message:       sampleMsg,
	OptionalCount: 0,
	Internal:      false,
	Handler:       nil,
}

var sampleRegistry = schema.MessageDescriptorRegistry{}

var sampleBuf = []byte{
	0x62, 0x75, 0x66, 0x66, 0x65, 0x72, // fixed
	0x06, 0x00, 0x62, 0x75, 0x66, 0x66, 0x65, 0x72, // dynamic
	0x06, 0x00, 0x00, 0x00, 0x62, 0x75, 0x66, 0x66, 0x65, 0x72, // dynamic long
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // uint64
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // int64
	0xff, 0xff, 0xff, 0xff, // uint32
	0xff, 0xff, 0xff, 0xff, // int32
	0xff, 0xff, // uint16
	0xff, 0xff, // int16
	0x02, 0x00,
	0x62, 0x28,
	0x26, 0x82,
}

var sampleBuf2 = []byte{
	0x00, 0x00, 0x00, 0x00, // minVersion                (0)
	0x00, 0x00, 0x00, 0x00, // currVersion               (0)
	0x01, 0x00, // schema [length]           (1)

	// schema[0]
	0x02, 0x00, 0x00, 0x00, // schema[0].id              (2)
	0x01, 0x00, //             schema[0].internal        (false)
	0x01, 0x00, //             schema[0].direction       (outbound)
	0x0d, 0x00, //             schema[0].name [length]   (13)
	// schema[0].name (ProtocolError)
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x45, 0x72, 0x72, 0x6f, 0x72,
	0x01, 0x00, // schema[0].fields [length] (1)

	// schema[0].fields[0]
	0x07, 0x00, //   schema[0].fields[0].name [length]   (7)
	// schema[0].fields[0].name (message)
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x01, 0x00, //   schema[0].fields[0].type            (TypeDynamicBinary)
	0x00, 0x00, 0x00, 0x00, // fields[0].extra [length]  (7)
	0x00, 0x00, // fields[0].optional                    (false)
}

var expectedBin = []byte{
	0x62, 0x75, 0x66, 0x66, 0x65, 0x72,
}

var expectedU64 uint64 = 18446744073709551615
var expectedI64 int64 = -1

var expectedU32 uint32 = 4294967295
var expectedI32 int32 = -1

var expectedU16 uint16 = 65535
var expectedI16 int16 = -1

func validateRes(res sampleStruct) error {
	if !slices.Equal(res.Fixed, expectedBin) {
		return fmt.Errorf("invalid fixed")
	}

	if !slices.Equal(res.Long, expectedBin) {
		return fmt.Errorf("invalid long")
	}

	if !slices.Equal(res.Dynamic, expectedBin) {
		return fmt.Errorf("invalid dynamic")
	}

	if res.UInt64 != expectedU64 {
		return fmt.Errorf("invalid uint64")
	}

	if res.Int64 != expectedI64 {
		return fmt.Errorf("invalid int64")
	}

	if res.UInt32 != expectedU32 {
		return fmt.Errorf("invalid uint32")
	}

	if res.Int32 != expectedI32 {
		return fmt.Errorf("invalid int32")
	}

	if res.UInt16 != expectedU16 {
		return fmt.Errorf("invalid uint16")
	}

	if res.Int16 != expectedI16 {
		return fmt.Errorf("invalid int16")
	}

	return nil
}

func TestA(t *testing.T) {
	sampleRegistry.RegisterInternal()

	sampleDesc2 := sampleRegistry.Descriptors[1]

	var res helloOutbound

	reader := NewReader(sampleBuf2, sampleDesc2)
	err := reader.Decode(&res)

	if err != nil {
		t.Error(err)
	}

	/*err = validateRes(res)

	if err != nil {
		t.Error(err)
	}*/

	log.Printf("%+v\n", res)
}

// make sure to run this test with checking pointer and GC
func TestB(t *testing.T) {
	var res sampleStruct

	reader := exp.NewReader(sampleBuf)
	err := reader.Decode(sampleDesc, &res)

	if err != nil {
		t.Error(err)
	}

	err = validateRes(res)

	if err != nil {
		t.Error(err)
	}

	log.Printf("%+v\n", res)
}

func TestC(t *testing.T) {
	var res sampleStruct

	reader := NewReader(sampleBuf, sampleDesc)
	err := reader.Decode(&res)

	if err != nil {
		t.Error(err)
	}

	/*err = validateRes(res)

	if err != nil {
		t.Error(err)
	}*/

	log.Printf("%+v\n", res)
}

func BenchmarkDecodeA(b *testing.B) {
	sampleRegistry.RegisterInternal()

	sampleDesc2 := sampleRegistry.Descriptors[1]

	for i := 0; i < b.N; i++ {
		var res helloOutbound

		reader := NewReader(sampleBuf2, sampleDesc2)
		err := reader.Decode(&res)

		if err != nil {
			b.Error(err)
		}

		/*err = validateRes(res)

		if err != nil {
			b.Error(err)
		}*/
	}
}

func BenchmarkDecodeB(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var res sampleStruct

		reader := exp.NewReader(sampleBuf)
		err := reader.Decode(sampleDesc, &res)

		if err != nil {
			b.Error(err)
		}

		err = validateRes(res)

		if err != nil {
			b.Error(err)
		}
	}
}
