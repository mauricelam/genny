// This file was automatically generated by genny.
// Any changes will be lost if this file is regenerated.
// see https://github.com/CanDIG/genny

package main

import "github.com/CanDIG/genny/examples/user-defined-types/person"
import "github.com/CanDIG/genny/examples/user-defined-types/pet"

type PairPersonDog struct {
	First  person.Person
	Second pet.Dog
}

func (p PairPersonDog) Left() person.Person {
	return p.First
}

func (p PairPersonDog) Right() pet.Dog {
	return p.Second
}
