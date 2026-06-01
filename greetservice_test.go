package main

import "testing"

func TestGreetService_Greet(t *testing.T) {
	s := &GreetService{}
	got := s.Greet("World")
	want := "Hello World!"
	if got != want {
		t.Errorf("Greet() = %q, want %q", got, want)
	}
}

func TestGreetService_GreetEmpty(t *testing.T) {
	s := &GreetService{}
	got := s.Greet("")
	want := "Hello !"
	if got != want {
		t.Errorf("Greet() = %q, want %q", got, want)
	}
}
