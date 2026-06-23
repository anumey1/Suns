package plist

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"time"
)

// ErrStalled means a complete document did not arrive within the read-deadline:
// powermetrics stalled mid-flush (a partial document with no closing tag and no
// EOF). The supervisor treats this as a first-class state distinct from EOF and
// restarts the subprocess (§7.3, §7.4).
var ErrStalled = errors.New("plist: stream stalled (read-deadline exceeded)")

var plistClose = []byte("</plist>")

// SplitDocuments is a bufio.SplitFunc that yields one complete plist document at
// a time from a concatenated powermetrics stream. `powermetrics -f plist`
// emits a sequence of discrete <?xml … </plist> documents (sometimes separated
// by a NUL); a single-document decoder fails at the first boundary, so each
// document is buffered whole and handed to the decoder individually (§7.3).
func SplitDocuments(data []byte, atEOF bool) (advance int, token []byte, err error) {
	idx := bytes.Index(data, plistClose)
	if idx < 0 {
		if atEOF && len(bytes.TrimSpace(data)) > 0 {
			// Trailing bytes with no closing tag at EOF: not a complete doc.
			return len(data), nil, nil
		}
		return 0, nil, nil // need more data
	}
	end := idx + len(plistClose)
	// Consume a trailing NUL and/or newline separator if present.
	for end < len(data) && (data[end] == 0x00 || data[end] == '\n' || data[end] == '\r') {
		end++
	}
	return end, bytes.TrimSpace(data[:idx+len(plistClose)]), nil
}

// StreamWithDeadline reads complete plist documents from r and calls onDoc for
// each. If no complete document arrives within deadline, it invokes onStall
// (which the supervisor uses to kill/restart the subprocess, since a goroutine
// blocked in an in-kernel read cannot be cancelled — closing the pipe via
// onStall unblocks it) and returns ErrStalled. It returns nil on EOF and the
// ctx error if cancelled.
func StreamWithDeadline(ctx context.Context, r io.Reader, deadline time.Duration, onDoc func([]byte), onStall func()) error {
	type item struct {
		doc []byte
		err error
	}
	ch := make(chan item, 1)

	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		sc.Split(SplitDocuments)
		for sc.Scan() {
			b := append([]byte(nil), sc.Bytes()...) // copy: scanner reuses its buffer
			ch <- item{doc: b}
		}
		ch <- item{err: sc.Err()} // nil err ⇒ EOF
	}()

	timer := time.NewTimer(deadline)
	defer timer.Stop()
	for {
		timer.Reset(deadline)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if onStall != nil {
				onStall()
			}
			return ErrStalled
		case it := <-ch:
			if it.err != nil {
				return it.err
			}
			if it.doc == nil {
				return nil // EOF
			}
			if onDoc != nil {
				onDoc(it.doc)
			}
		}
	}
}
