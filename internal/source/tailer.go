package source

import (
	"bufio"
	"io"
	"os"
	"sync"
)

const maxLineSize = 1 << 20

type Tailer struct {
	mu            sync.Mutex
	path          string
	offset        int64
	buf           []byte
	skipToNewline bool
}

func NewTailer(path string) *Tailer {
	return &Tailer{path: path}
}

func (t *Tailer) SeekEnd() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, err := os.Stat(t.path)
	if err != nil {
		return err
	}
	t.offset = info.Size()
	return nil
}

func (t *Tailer) ReadNewLines() ([][]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	linfo, err := os.Lstat(t.path)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil
	}

	f, err := os.Open(t.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size() < t.offset {
		t.offset = 0
		t.buf = nil
	}

	if info.Size() == t.offset && len(t.buf) == 0 {
		return nil, nil
	}

	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(f)
	var lines [][]byte

	var current []byte
	if len(t.buf) > 0 {
		current = append(current, t.buf...)
		t.buf = nil
	}

	for {
		chunk, err := reader.ReadBytes('\n')
		current = append(current, chunk...)

		if err == io.EOF {
			if len(current) > 0 && len(current) <= maxLineSize {
				t.buf = current
			} else if len(current) > maxLineSize {
				t.buf = nil
				t.skipToNewline = true
			}
			break
		}
		if err != nil {
			pos, seekErr := f.Seek(0, io.SeekCurrent)
			if seekErr == nil {
				t.offset = pos
			}
			return lines, err
		}

		if t.skipToNewline {
			t.skipToNewline = false
			current = current[:0]
			continue
		}

		if len(current) > maxLineSize {
			current = current[:0]
			continue
		}

		line := make([]byte, len(current))
		copy(line, current)
		lines = append(lines, line)
		current = current[:0]
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		t.offset = pos
	}

	return lines, nil
}

func (t *Tailer) Offset() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.offset
}
