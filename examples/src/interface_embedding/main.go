package main

type InfA interface {
	A1()
}

type InfB interface {
	B1()
}

type Inf interface {
	InfA
	InfB
}

type A struct{}

func (a A) A1() {
	println("A's A1 called")
}

func (a A) B1() {
	println("A's B1 called")
}

type C struct {
	A
}

func (c C) A1() {
	println("C's A1 called")
}

type D struct {
	A
}

func test(inf Inf) {
	inf.A1()
}

func main() {
	var c C
	println("c call c.A1(), c.B1()")
	c.A1()
	c.B1()

	var d D
	println("d call d.A1(), d.B1()")
	d.A1()
	d.B1()
	test(c)
}
