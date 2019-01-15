package at

// sn means Struct Notation

import (
	"fmt"
	"unsafe"
)

type Attribute map[string]interface{}

func (me Attribute) Get(key string, defaultValue interface{}) interface{} {
	if attr, ok := me[key]; ok {
		return attr
	}
	return defaultValue
}

func (me Attribute) Set(key string, value interface{}) {
	me[key] = value
}

// TODO:: 这样就需要额外的变量，不如直接使用 Struct
// func (me Attribute) SetReadonly

func (me Attribute) GetAttribute(key string, defaultValue Attribute) Attribute {
	if attr, ok := me[key]; ok {
		switch t := attr.(type) {
		case Attribute:
			return t
		case map[string]interface{}:
			return *(*Attribute)(unsafe.Pointer(&t))
		default:
			// TODO:: log.E()
			fmt.Printf("Error: unsupport convert %T to Attribute, use default value instead\n", t)
		}
	}
	return defaultValue
}

func (me Attribute) GetString(key string, defaultValue string) string {
	if attr, ok := me[key]; ok {
		switch t := attr.(type) {
		case string:
			return t
		case rune, byte, uint, uint16, uint32, uint64, int, int8, int16, int64, float32, float64: //byte = uint8, rune = int32
			return fmt.Sprintf("%s", t)
		default:
			fmt.Printf("Error: unsupport convert %s %T:%s to string, use default value instead\n", key, t, t)
		}
	}
	return defaultValue
}

func (me Attribute) GetArray(key string, defaultValue []interface{}) []interface{} {
	if attr, ok := me[key]; ok {
		switch t := attr.(type) {
		case []interface{}:
			return t
		default:
			fmt.Printf("Error: unsupport convert %s %T:%s to array, use default value instead\n", key, t, t)
		}
	}
	return defaultValue
}
