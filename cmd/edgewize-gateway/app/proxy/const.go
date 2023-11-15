package proxy

import "time"

const (
	MaxIdleConns          = 100
	IdleConnTimeout       = 90 * time.Second
	ExpectContinueTimeout = 1 * time.Second
	MaxConnsPerHost       = 30
)
