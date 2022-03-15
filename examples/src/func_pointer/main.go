package main

type SomeInterface interface {
	b()
}

type A struct{}

func (a A) b() {
	println("A's b")
}

type S struct {
	a A
}

func main() {
	var s S
	test1(s.a.b)
	test2(s)
}

func test1(fn func()) {
	println("test1")
	fn()
}

func test2(s S) {
	s.a.b()
}
