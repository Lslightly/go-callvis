package main

type Inf interface {
	f()
}

type S struct{}
type T struct{}

func (s S) f() {
	println("S's f()")
}

func (t T) f() {
	println("T's f()")
}

func passInf(inf *Inf) *Inf {
	(*inf).f()
	return inf
}

func main() {
	var inf Inf = &S{}
	(*passInf(&inf)).f()
	ch1 := make(chan bool)
	ch2 := make(chan bool)
	go func() {
		inf.f()
		ch1 <- true
	}()
	go func() {
		(*passInf(&inf)).f()
		ch2 <- true
	}()
	<-ch1
	<-ch2
	inf = &T{}
	inf.f()
}
