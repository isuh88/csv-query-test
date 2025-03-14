package main

import (
	"fmt"
	"time"
)

type TimeCheck struct {
	start time.Time
}

func NewTimeCheck() *TimeCheck {
	t := &TimeCheck{
		start: time.Now(),
	}
	fmt.Printf("[START: %s] Request started\n", t.start.Format("2006-01-02 15:04:05.000"))
	return t
}

func (t *TimeCheck) End() {
	end := time.Now()
	elapsed := end.Sub(t.start)
	fmt.Printf("[END: %s] Request completed (took %v)\n", end.Format("2006-01-02 15:04:05.000"), elapsed)
}
