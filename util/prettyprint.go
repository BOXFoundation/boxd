// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"

	"github.com/BOXFoundation/boxd/crypto"
)

// PrettyPrint prints all types with pretty ident and format
func PrettyPrint(obj interface{}) string {
	val := reflect.ValueOf(obj)

	// type exceptions
	switch val.Type() {
	case hashType:
		return obj.(crypto.HashType).String()
	}

	// basic types
	switch val.Type().Kind() {
	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uint8, reflect.String, reflect.Int, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Int8, reflect.Float32,
		reflect.Float64, reflect.Complex128, reflect.Complex64:
		return fmt.Sprint(obj)
	case reflect.Ptr, reflect.Interface:
		return PrettyPrint(printValue(val.Elem()))
	case reflect.Array, reflect.Slice:
		return arrayPrint(obj)
	case reflect.Struct:
		return structPrint(val.Interface())
	case reflect.Map:
		return mapPrint(val.Interface())
	case reflect.Chan, reflect.UnsafePointer:
		return ""
	default:
		return fmt.Sprint(obj)
	}
}

func printValue(v reflect.Value) string {
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return "Nil"
	}
	if v.CanInterface() {
		return PrettyPrint(v.Interface())
	}
	return ""
}

var hash crypto.HashType
var hashType = reflect.TypeOf(hash)

func arrayPrint(obj interface{}) string {
	value := reflect.ValueOf(obj)
	toPrint := "Array"
	if value.Len() > 0 && value.Index(0).Kind() == reflect.Uint8 {
		return fmt.Sprintf("0x%x", hex.EncodeToString(obj.([]byte)))
	}
	for i := 0; i < value.Len(); i++ {
		v := value.Index(i)
		var str string
		if v.CanInterface() {
			str = PrettyPrint(v.Interface())
		} else {
			str = v.String()
		}
		toPrint += "\n"
		toPrint += fmt.Sprintf("%d\t:%s", i, str)
	}
	return toPrint
}

func structPrint(obj interface{}) string {
	value := reflect.ValueOf(obj)
	t := value.Type()
	toPrint := t.Name()
	for i := 0; i < value.NumField(); i++ {
		fieldValue := value.Field(i)
		if !fieldValue.CanInterface() {
			continue
		}
		fieldStr := printValue(fieldValue)
		toPrint += "\n"
		toPrint += addIdent(fmt.Sprintf("%s:\t%s", t.Field(i).Name, fieldStr))
	}
	return toPrint
}

func mapPrint(obj interface{}) string {
	value := reflect.ValueOf(obj)
	toPrint := "Map"
	keys := value.MapKeys()
	for _, key := range keys {
		val := value.MapIndex(key)
		toPrint += "\n"
		keyStr := printValue(key)
		valStr := printValue(val)
		toPrint += fmt.Sprintf("%s:\t%s", keyStr, valStr)
	}
	return toPrint
}

func addIdent(str string) string {
	return strings.Replace(str, "\n", "\n--", -1)
}
