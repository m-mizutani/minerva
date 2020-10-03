package main

import (
	"math"
	"time"
)

type retryTimer struct {
	retryCount int
}

func (x *retryTimer) sleep() {
	waitTime := x.calcWaitTime()
	time.Sleep(waitTime)
}

func (x *retryTimer) calcWaitTime() time.Duration {
	wait := math.Pow(2.0, float64(x.retryCount))/64 + 0.5
	if wait > 2 {
		wait = 2
	}
	mSec := time.Millisecond * time.Duration(wait*1000)
	x.retryCount++
	return mSec
}

func (x *retryTimer) clear() {
	x.retryCount = 0
}
