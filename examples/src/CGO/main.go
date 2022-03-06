package main

/*
#include "test.h"
*/
import "C"

func main() {
	C.test()
	C.GO2C()
	C2GO()
}
