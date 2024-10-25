package internal

import "fmt"

type stringer string

func (s stringer) String() string {
	return string(s)
}

func NewStringer(val string) fmt.Stringer {
	return stringer(val)
}
