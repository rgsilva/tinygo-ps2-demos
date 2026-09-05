package clock

/*
#include <time.h>
*/
import "C"

const (
	CLOCKS_PER_SEC = 1000000
)

func Now() uint64 {
	return uint64(C.clock())
}
