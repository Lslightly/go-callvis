package main

/*
#include "test.h"
*/
import "C"

//export C2GO
func C2GO() {
	println("c call go succeeds")
}

func call_GO2C() {
	C.GO2C()
}
