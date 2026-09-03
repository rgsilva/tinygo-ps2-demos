//go:build ps2go_sched

// Goroutine and channel cases. The ps2 target has no scheduler yet
// (roadmap: scheduler improvements); build with -tags ps2go_sched once it does.
package main

import (
	"fmt"
	"runtime"
	"sync"

	"ps2go/harness"
)

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

func init() {
	extraCases = append(extraCases,
		harness.Case{Name: "goroutines", Fn: testGoroutines},
		harness.Case{Name: "channels", Fn: testChannels},
	)
}
