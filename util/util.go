package util

import (
	"runtime"
)

func Unclog[T any](c chan T) {
	//non-blockingly receives msgs from channel until none left
	for {
		select {
		case <-c:
			continue
		default:
			return
		}
	}
}


func SetRuntimeBlockProfileRate(rate int){
	runtime.SetBlockProfileRate(rate)
}

func SetRuntimeMutexProfileRate(rate int){
	runtime.SetMutexProfileFraction(rate)
}