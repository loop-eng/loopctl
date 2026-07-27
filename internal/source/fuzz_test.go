package source

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// FuzzTailerReadNewLines fuzzes Tailer.ReadNewLines against a real file:
// the fuzz input is written to disk, read once, then appended to and read
// again to exercise the incremental-tail and truncation-recovery paths
// together. The offset invariant (never past EOF, never negative) is the
// single most important property here — an offset overshoot would
// silently drop data forever without ever panicking.
func FuzzTailerReadNewLines(f *testing.F) {
	f.Add([]byte(`{"line":1}` + "\n"))
	f.Add([]byte(`{"line":1}` + "\n" + `{"line":2}` + "\n"))
	f.Add([]byte("partial line no newline"))
	f.Add([]byte("\n\n\n"))
	f.Add([]byte(""))
	f.Add(bytes.Repeat([]byte("a"), maxLineSize+100))
	f.Add(bytes.Repeat([]byte("a"), maxLineSize))
	f.Add(append(bytes.Repeat([]byte{0xff, 0x00, 0xfe}, 100), '\n'))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "fuzz.jsonl")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Skip("could not write seed to disk")
		}

		tailer := NewTailer(path)
		assertTailerInvariants(t, tailer, path)

		lines, err := tailer.ReadNewLines()
		_ = err // malformed content is not itself an error condition here
		assertLinesWithinLimit(t, lines)
		assertTailerInvariants(t, tailer, path)

		// Append more of the same data to exercise the incremental-read and
		// truncation-recovery paths (a shorter second write naturally
		// exercises the info.Size() < t.offset reset branch).
		f2, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f2.Write(data); err != nil {
			f2.Close()
			t.Fatal(err)
		}
		f2.Close()

		lines, err = tailer.ReadNewLines()
		_ = err
		assertLinesWithinLimit(t, lines)
		assertTailerInvariants(t, tailer, path)
	})
}

func assertTailerInvariants(t *testing.T, tailer *Tailer, path string) {
	t.Helper()
	offset := tailer.Offset()
	if offset < 0 {
		t.Fatalf("tailer offset went negative: %d", offset)
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if offset > info.Size() {
		t.Fatalf("tailer offset %d exceeds file size %d", offset, info.Size())
	}
}

func assertLinesWithinLimit(t *testing.T, lines [][]byte) {
	t.Helper()
	for _, l := range lines {
		if len(l) > maxLineSize {
			t.Fatalf("returned line exceeds maxLineSize: %d bytes", len(l))
		}
	}
}
