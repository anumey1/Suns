package trash

import "sync"

// breaker is the circuit breaker that, after a threshold of consecutive native
// trash-call timeouts, stops attempting the native (Cgo) path so that callers
// degrade to the pure-Go fallback (§4.4). Crucially it DEGRADES rather than
// refusing: when tripped, Trasher.Trash still moves the item via the fallback,
// so a healthy local volume keeps working while a dead mount is bypassed.
//
// A success resets the consecutive-timeout count and re-closes the breaker, so
// a transient stall does not permanently disable the native path.
type breaker struct {
	mu        sync.Mutex
	threshold int
	consec    int
	tripped   bool
}

func newBreaker(threshold int) *breaker {
	return &breaker{threshold: threshold}
}

// allow reports whether the native call should be attempted.
func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.tripped
}

func (b *breaker) recordTimeout() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consec++
	if b.consec >= b.threshold {
		b.tripped = true
	}
}

func (b *breaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consec = 0
	b.tripped = false
}
