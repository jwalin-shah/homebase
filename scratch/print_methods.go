package main

import (
	"fmt"
	"reflect"
	"github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"
)

func main() {
	var seq dafny.Sequence
	t := reflect.TypeOf(&seq).Elem()
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Printf("Method %s: %s\n", m.Name, m.Type)
	}
}
