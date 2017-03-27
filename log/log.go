package log

import (
	"bufio"
	"io"
	stdLog "log"
	"strings"
)

func Printf(format string, a ...interface{}) {
	stdLog.Printf(format, a...)
}

func Fatalf(format string, a ...interface{}) {
	stdLog.Fatalf(format, a...)
}

func PrintLinesPrefixed(prefix, lines string) {
	r := bufio.NewReader(strings.NewReader(lines))
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				Printf("%s %s", prefix, string(line))
			}
			break
		}
		Printf("%s %s", prefix, string(line))
	}
}
