package encoder

import (
	"reflect"
	"sync/atomic"
	"unsafe"
)

var ptrOffset uintptr = 0
var hasPtrOffset atomic.Bool

func getValuePtr(v *reflect.Value) unsafe.Pointer {
	if v == nil || !v.IsValid() {
		return nil
	}

	// need to ensure better concurrency handling
	if !hasPtrOffset.Swap(true) {
		t := reflect.TypeOf(reflect.Value{})
		f, exists := t.FieldByName("ptr")

		if !exists {
			panic("reflect.Value.ptr doesn't exist")
		}

		if f.Type.Kind() != reflect.UnsafePointer {
			panic("reflect.Value.ptr isn't a unsafe.Pointer, must update code")
		}

		ptrOffset = f.Offset
	}

	basePtr := uintptr(unsafe.Pointer(v))
	targetPtr := unsafe.Pointer(basePtr + ptrOffset)

	return *(*unsafe.Pointer)(targetPtr)
}