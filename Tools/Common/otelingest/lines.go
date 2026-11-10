package otelingest

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
)

// scanned is what one read of the OTel file produced.
type scanned struct {
	lines     []parsedLine
	read      int
	malformed int
	end       int64 // offset after the last complete line consumed
}

// scanFile reads the complete lines of the file from start. A trailing partial line is not read and not consumed.
// Lines without the usage marker are skipped without JSON parsing.
func scanFile(path string, start int64) (scanned, error) {
	out := scanned{end: start}
	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return out, err
	}
	r := bufio.NewReaderSize(f, 1<<20)
	if start > 0 && !followsNewline(f, start) {
		skipped, err := r.ReadBytes('\n')
		if err != nil {
			return out, nil
		}
		out.end += int64(len(skipped))
	}
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
		offset := out.end
		out.end += int64(len(line))
		out.read++
		if !bytes.Contains(line, usageMarker) {
			continue
		}
		p := parseLine(line, offset)
		if p.malformed {
			out.malformed++
			continue
		}
		if p.link != nil || p.call != nil || p.tool != nil {
			out.lines = append(out.lines, p)
		}
	}
}

// followsNewline reports whether the byte before offset is a newline, i.e. offset is at the start of a line.
func followsNewline(f *os.File, offset int64) bool {
	b := make([]byte, 1)
	_, err := f.ReadAt(b, offset-1)
	return err == nil && b[0] == '\n'
}
