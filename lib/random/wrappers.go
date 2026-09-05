package random

/*
#define _EE
#include <stdlib.h>
#include <tamtypes.h>
#include <kernel.h>
#include <timer.h>

void random_init() {
	srand(GetTimerCount(0));
}

int random_get() {
	return rand();
}
*/
import "C"

func Init() {
	C.random_init()
}

func Next() int {
	return int(C.random_get())
}

func Between(min, max int) int {
	return min + (Next() % (max - min + 1))
}
