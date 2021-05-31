package godepi

import (
	"fmt"
	"reflect"
)

func getDepName(dep interface{}) string {
	switch dep.(type) {
	case string:
		// TODO add checking of depName existence
		return dep.(string)

	default:
		nType := reflect.TypeOf(dep)
		// TODO add fields count check
		nField := nType.Field(0)

		return getFullDepPath(nField.Type)
	}
}

func getFullDepPath(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t.PkgPath() + "." + t.Name()
}

func checkFactory(dep interface{}, f FactoryFunc) {
	if getFactoryReturnDepName(f) != getDepName(dep) {
		panic(fmt.Sprintf(`wrong return type in factory for "%s" dependency, got "%s"`, getDepName(dep), getFactoryReturnDepName(f)))
	}
}

func getFactoryReturnDepName(f FactoryFunc) string {
	fType := reflect.TypeOf(f)
	// TODO add fields count check
	retType := fType.Out(0)

	return getFullDepPath(retType)
}
