package evalgrade

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

const maxRecordLine = 1 << 20

// TypeScriptRun is the outcome of one node --test invocation: what the reporter recorded, the records it could not
// parse, and everything the run wrote to standard output and error.
type TypeScriptRun struct {
	Records   []TestRecord
	Malformed int
	Output    string
}

// readRecords parses one record per line. A line that is not a record, or is longer than any record, counts as
// malformed and costs only itself: the lines after it are still read.
func readRecords(source io.Reader) TypeScriptRun {
	var run TypeScriptRun
	reader := bufio.NewReaderSize(source, maxRecordLine)
	for {
		line, oversize, err := nextLine(reader)
		if len(line) > 0 || oversize {
			run.add(line, oversize)
		}
		if err != nil {
			return run
		}
	}
}

func (run *TypeScriptRun) add(line []byte, oversize bool) {
	var record TestRecord
	if oversize || json.Unmarshal(line, &record) != nil {
		run.Malformed++
		return
	}
	run.Records = append(run.Records, record)
}

func nextLine(reader *bufio.Reader) (line []byte, oversize bool, err error) {
	line, err = reader.ReadSlice('\n')
	for errors.Is(err, bufio.ErrBufferFull) {
		oversize = true
		_, err = reader.ReadSlice('\n')
	}
	return line, oversize, err
}
