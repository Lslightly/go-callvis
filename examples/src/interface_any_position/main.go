package main

type Inf interface {
	f()
}

type S struct{}

func (s S) f() {
	println("S's f()")
}

func passInf(inf *Inf) *Inf {
	(*inf).f()
	return inf
}

func main() {
	var inf Inf = &S{}
	(*passInf(&inf)).f()
	ch := make(chan bool)
	go func() {
		inf.f()
		ch <- true
	}()
	<-ch
}
