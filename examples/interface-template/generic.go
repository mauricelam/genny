package main

import (
	"fmt"

	"github.com/olegch/genny/generic"
)

//go:generate genny -pkg=main -in=generic.go -out=gen-$GOFILE gen "TypeParam=string"

// TypeParam parameter type
type TypeParam generic.Type

// PrinterTypeParamInterface parameter type printer interface
type PrinterTypeParamInterface interface {
	Print(value TypeParam) string
}

// PrinterTypeParam parameter type printer
type PrinterTypeParam struct {
}

// Print generates string from an object
func (p *PrinterTypeParam) Print(value TypeParam) string {
	return fmt.Sprintf("%v", value)
}
