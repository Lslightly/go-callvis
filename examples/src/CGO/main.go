package main

/*
#include "test.h"
*/
import "C"

func main() {
	C.test()
	C2GO()
}
