package plist_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/anumey1/Suns/pkg/plist"
)

const doc1 = `<?xml version="1.0"?><plist version="1.0"><dict><key>a</key><integer>1</integer></dict></plist>`
const doc2 = `<?xml version="1.0"?><plist version="1.0"><dict><key>b</key><integer>2</integer></dict></plist>`

func TestSplitDocuments_ConcatenatedStream(t *testing.T) {
	// powermetrics emits documents back-to-back, sometimes NUL-separated.
	stream := doc1 + "\x00" + doc2 + "\n"
	sc := bufio.NewScanner(strings.NewReader(stream))
	sc.Split(plist.SplitDocuments)

	var got []string
	for sc.Scan() {
		got = append(got, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d documents, want 2: %q", len(got), got)
	}
	if !strings.HasSuffix(got[0], "</plist>") || !strings.HasSuffix(got[1], "</plist>") {
		t.Fatalf("documents not split on </plist>: %q", got)
	}
	if strings.Contains(got[0], "key>b<") {
		t.Fatalf("first document leaked into second: %q", got[0])
	}
}

func TestStreamWithDeadline_DeliversDocs(t *testing.T) {
	r := strings.NewReader(doc1 + doc2)
	var n int
	err := plist.StreamWithDeadline(context.Background(), r, time.Second, func([]byte) { n++ }, nil)
	if err != nil {
		t.Fatalf("StreamWithDeadline: %v", err)
	}
	if n != 2 {
		t.Fatalf("delivered %d docs, want 2", n)
	}
}

// A producer that stalls mid-document (no closing tag, no EOF) must trip the
// read-deadline → ErrStalled and the onStall callback (§7.3).
func TestStreamWithDeadline_StallTripsDeadline(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		// Write a partial document, then hang (never close).
		_, _ = pw.Write([]byte(`<?xml version="1.0"?><plist><dict>`))
		select {} // block forever
	}()

	var stalled bool
	err := plist.StreamWithDeadline(context.Background(), pr, 150*time.Millisecond,
		func([]byte) { t.Error("no complete document should be delivered") },
		func() { stalled = true; _ = pr.Close() }, // closing unblocks the reader
	)
	if !errors.Is(err, plist.ErrStalled) {
		t.Fatalf("err = %v, want ErrStalled", err)
	}
	if !stalled {
		t.Fatal("onStall was not invoked")
	}
}
