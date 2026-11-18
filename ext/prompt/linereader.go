package prompt

import "io"

// lineReader reads input one line at a time without reading past the
// delimiter, so successive prompts (and other code) can share one input.
// A bufio.Reader per prompt would buffer, and then discard, the answers
// meant for later prompts.
type lineReader struct {
	r io.Reader
}

// newLineReader returns a lineReader for r.
func newLineReader(r io.Reader) lineReader {
	return lineReader{r: r}
}

// maxEmptyReads bounds how many consecutive (0, nil) reads are tolerated
// before giving up, mirroring bufio.
const maxEmptyReads = 100

// ReadString reads through the next delim and returns the data including
// it. A final line without delim is returned with a nil error; io.EOF is
// returned only when no data remains.
func (l lineReader) ReadString(delim byte) (string, error) {
	var buf []byte
	finish := func(err error) (string, error) {
		if err == io.EOF && len(buf) > 0 {
			return string(buf), nil
		}
		return string(buf), err
	}

	if br, ok := l.r.(io.ByteReader); ok {
		for {
			b, err := br.ReadByte()
			if err != nil {
				return finish(err)
			}
			buf = append(buf, b)
			if b == delim {
				return string(buf), nil
			}
		}
	}

	var one [1]byte
	for empty := 0; ; {
		n, err := l.r.Read(one[:])
		if n > 0 {
			empty = 0
			buf = append(buf, one[0])
			if one[0] == delim {
				return string(buf), nil
			}
		}
		if err != nil {
			return finish(err)
		}
		if n == 0 {
			if empty++; empty >= maxEmptyReads {
				return finish(io.ErrNoProgress)
			}
		}
	}
}
