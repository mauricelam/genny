package join

import (
	"fmt"

	"github.com/CanDIG/genny/generic"
)

type Stringer interface {
	// Comments for generic.Type should be removed
	generic.Type
	fmt.Stringer
}

func JoinStringers(list []Stringer, sep string) (result string) {
	for i, elem := range list {
		if i > 0 {
			result += sep
		}
		result += elem.String()
	}
	return
}
