package syntax

import (
    "github.com/CanDIG/genny/generic"
)

type myType generic.Type
type myTypeList []myType

type MyTypeUppercase myType

var _ myType
var myTypeVariable string

func _() {
	var _ []myType  // A comment
	var _ myTypeList
	var _ []myTypeList
}

func PrintMyType(_myType myType) {
	var _ MyTypeUppercase

	println(myTypeVariable, _myType, myType(123))
}
