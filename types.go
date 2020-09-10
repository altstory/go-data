package data

import (
	"reflect"
	"time"
)

var (
	typeOfBool       = reflect.TypeOf(true)
	typeOfInt64      = reflect.TypeOf(int64(0))
	typeOfUint64     = reflect.TypeOf(uint64(0))
	typeOfFloat64    = reflect.TypeOf(float64(0))
	typeOfComplex128 = reflect.TypeOf(complex128(0))
	typeOfString     = reflect.TypeOf("")
	typeOfObject     = reflect.TypeOf(RawData{})
	typeOfInterface  = reflect.TypeOf((*interface{})(nil)).Elem()
	typeOfData       = reflect.TypeOf(Data{})
	typeOfTime       = reflect.TypeOf(time.Time{})
	typeOfDuration   = reflect.TypeOf(time.Duration(0))
)
