package main

import (
	"math"
	"time"
)

const (
	notificationScriptTimeout = 60
	verificationScriptTimeoiut = 60
	INTERVAL = 60
	GOSSIP_LEAVE_INTERVAL_IN_SECONDS = 3
	STALE_INFO_INTERVAL_IN_SECONDS = 300
)

const (
	verificationFormat = "%s %s %s"
	notificationFormat = "%s %s %s %s"
)

func GoRoutineInSleep(duration time.Duration, function func()) {
	go func() {
		for {
			function()
			time.Sleep(duration)
		}
	}()
}

func GoRoutineInTimeScale(base int64,f1 func() int64, function func()) {
	go func() {
		for {
			function()
			length := math.Ceil(math.Log10(float64(Max(f1(),2))))
			scale := math.Exp2(length)
			interval := base/int64(scale)
			interval = Max(interval,3)
			time.Sleep(time.Second*time.Duration(interval))
		}
	}()
}

func Max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func SliceEqual(a []string, b []string) bool {
	// If one is nil, the other must also be nil.
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

