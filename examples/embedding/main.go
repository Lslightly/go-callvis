package main

type I interface {
	m()
	p()
}

type A struct{}

type B struct {
	A
}

type C struct {
	A
}

type D struct {
	B
}

type E struct {
	C
}

type F struct {
	C
}

type G struct {
	F
}

type H struct {
	F
}

func (A) m() {
	println("A's m")
}

func (A) p() {
	println("A's p")
}

func (B) m() {
	println("B's m")
}

func (C) m() {
	println("C's m")
}

func (E) m() {
	println("E's m")
}

func (F) p() {
	println("F's p")
}

func interface_call(a I) {
	a.m()
	a.p()
}

func main() {
	var g G
	interface_call(g)
	all()
}

func all() {
	var a A
	var b B
	var c C
	var d D
	var e E
	var f F
	var g G
	var h H
	interface_call(a)
	interface_call(b)
	interface_call(c)
	interface_call(d)
	interface_call(e)
	interface_call(f)
	interface_call(g)
	interface_call(h)
}
