package gotest

import (
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d", got)
	}
}

func TestGoroutines(t *testing.T) {
	done := make(chan int)
	go func() { time.Sleep(10 * time.Millisecond); done <- 42 }()
	if v := <-done; v != 42 {
		t.Fatalf("got %d", v)
	}
}
