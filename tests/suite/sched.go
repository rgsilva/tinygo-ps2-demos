package main

// Timer, interrupt and goroutine cases (tasks scheduler on the EE).

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"ps2go/lib/harness"
)

// testTimer checks the runtime clock: time.Sleep must actually wait and
// time.Now must advance monotonically.
func testTimer() error {
	t0 := time.Now()
	time.Sleep(10 * time.Millisecond)
	d10 := time.Since(t0)
	if d10 < 10*time.Millisecond || d10 > 100*time.Millisecond {
		return fmt.Errorf("slept %v for 10ms", d10)
	}
	t1 := time.Now()
	time.Sleep(250 * time.Millisecond)
	d250 := time.Since(t1)
	if d250 < 250*time.Millisecond || d250 > 400*time.Millisecond {
		return fmt.Errorf("slept %v for 250ms", d250)
	}
	prev := time.Now()
	for i := 0; i < 1000; i++ {
		now := time.Now()
		if now.Before(prev) {
			return fmt.Errorf("clock went backwards at %d", i)
		}
		prev = now
	}
	harness.Logf("timer: 10ms slept %v, 250ms slept %v", d10, d250)
	return nil
}

func testGoroutines() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				mu.Lock()
				total += g
				mu.Unlock()
				runtime.Gosched()
			}
		}(g)
	}
	wg.Wait()
	if total != 100*(0+1+2+3+4+5+6+7) {
		return fmt.Errorf("total %d", total)
	}
	return nil
}

func testChannels() error {
	in := make(chan int)
	out := make(chan int, 4)
	for w := 0; w < 4; w++ {
		go func() {
			for v := range in {
				out <- v * 2
			}
		}()
	}
	go func() {
		for i := 1; i <= 100; i++ {
			in <- i
		}
		close(in)
	}()
	sum := 0
	for i := 0; i < 100; i++ {
		sum += <-out
	}
	if sum != 2*100*101/2 {
		return fmt.Errorf("sum %d", sum)
	}
	sel := 0
	c := make(chan int, 1)
	select {
	case v := <-c:
		sel = v
	default:
		sel = -1
	}
	if sel != -1 {
		return fmt.Errorf("select default")
	}
	return nil
}

// --- Added on the exp/sched branch: harder scheduler cases. ---

type snode struct {
	v    int
	next *snode
}

// testSleepGoroutines: goroutines sleeping different durations must wake in
// deadline order and run concurrently (total time ~ the longest sleep).
func testSleepGoroutines() error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var order []int
	t0 := time.Now()
	for i := 5; i >= 1; i-- {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			time.Sleep(time.Duration(i) * 20 * time.Millisecond)
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	el := time.Since(t0)
	for i, v := range order {
		if v != i+1 {
			return fmt.Errorf("wake order %v", order)
		}
	}
	if el < 100*time.Millisecond || el > 250*time.Millisecond {
		return fmt.Errorf("elapsed %v for 5 concurrent sleeps (max 100ms)", el)
	}
	harness.Logf("sleep: 5 goroutines (20..100ms) done in %v", el)
	return nil
}

// testTimers: time.After in select, AfterFunc, Ticker, Timer.Stop.
func testTimers() error {
	c := make(chan int)
	select {
	case <-c:
		return fmt.Errorf("select on never-ready channel")
	case <-time.After(20 * time.Millisecond):
	}
	fired := make(chan int, 1)
	time.AfterFunc(10*time.Millisecond, func() { fired <- 42 })
	select {
	case v := <-fired:
		if v != 42 {
			return fmt.Errorf("AfterFunc value %d", v)
		}
	case <-time.After(200 * time.Millisecond):
		return fmt.Errorf("AfterFunc did not fire")
	}
	stopped := time.AfterFunc(10*time.Millisecond, func() { fired <- 1 })
	if !stopped.Stop() {
		return fmt.Errorf("Stop returned false")
	}
	tk := time.NewTicker(10 * time.Millisecond)
	t0 := time.Now()
	for i := 0; i < 5; i++ {
		<-tk.C
	}
	tk.Stop()
	el := time.Since(t0)
	if el < 50*time.Millisecond || el > 200*time.Millisecond {
		return fmt.Errorf("5 ticks of 10ms took %v", el)
	}
	select {
	case <-fired:
		return fmt.Errorf("stopped timer fired")
	default:
	}
	harness.Logf("timers: 5 ticks in %v", el)
	return nil
}

// testGCGoroutineStacks: pointers held only on goroutine stacks must survive
// collections triggered from other goroutines and from the goroutine itself.
func testGCGoroutineStacks() error {
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for g := 1; g <= 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			var head *snode
			for i := 0; i < 64; i++ {
				head = &snode{v: i * g, next: head}
				_ = make([]byte, 4096) // garbage
				if i%16 == 0 {
					runtime.GC()
				}
				runtime.Gosched()
			}
			i := 63
			for n := head; n != nil; n = n.next {
				if n.v != i*g {
					errs <- fmt.Errorf("goroutine %d: snode %d = %d", g, i, n.v)
					return
				}
				i--
			}
			if i != -1 {
				errs <- fmt.Errorf("goroutine %d: list length", g)
			}
		}(g)
	}
	wg.Wait()
	select {
	case err := <-errs:
		return err
	default:
	}
	return nil
}

//go:noinline
func yieldN(n int) {
	for i := 0; i < n; i++ {
		runtime.Gosched()
	}
}

// testCalleeSaved: float32/float64/int64 values live across task switches
// (kept in callee-saved s-/f-registers or spilled) must be preserved.
func testCalleeSaved() error {
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for g := 1; g <= 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			a := float32(g) * 1.5
			b := float32(g) * 0.25
			c := float64(g) * 3.75
			x := int64(g)<<40 | int64(g)
			y := uint64(0xdeadbeef)<<32 | uint64(g)
			for i := 0; i < 50; i++ {
				a += b
				c += 0.5
				x += 1 << 32
				y ^= uint64(i) << 33
				yieldN(3)
			}
			wa := float32(g)*1.5 + 50*float32(g)*0.25
			wc := float64(g)*3.75 + 25
			wx := int64(g)<<40 | int64(g) + 50<<32
			wy := uint64(0xdeadbeef)<<32 | uint64(g)
			for i := 0; i < 50; i++ {
				wy ^= uint64(i) << 33
			}
			if a != wa || c != wc || x != wx || y != wy {
				errs <- fmt.Errorf("goroutine %d: %v %v %x %x", g, a, c, x, y)
			}
		}(g)
	}
	wg.Wait()
	select {
	case err := <-errs:
		return err
	default:
	}
	return nil
}

// testProducerConsumer: unbuffered channel with sleeping producer and
// several consumers, plus a done channel closed by the last consumer.
func testProducerConsumer() error {
	in := make(chan int)
	var mu sync.Mutex
	sum := 0
	var wg sync.WaitGroup
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for v := range in {
				mu.Lock()
				sum += v
				mu.Unlock()
				time.Sleep(time.Millisecond)
			}
		}()
	}
	for i := 1; i <= 30; i++ {
		in <- i
	}
	close(in)
	wg.Wait()
	if sum != 30*31/2 {
		return fmt.Errorf("sum %d", sum)
	}
	return nil
}
