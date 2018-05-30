package golet

import (
	"fmt"
	"io"
	"time"
)

type Logger struct {
	enable      bool
	enableColor bool
	out         io.Writer
	sid         string
	clr         color
}

// Improved io.writer for golet
func (l *Logger) Write(data []byte) (n int, err error) {
	ln := len(data)
	if l.enable {
		startPos := 0
		for i := 0; i < ln; i++ {
			if data[i] == '\n' {
				l.write(data[startPos : i+1])
				startPos = i + 1
			}
		}
		l.write(append(data[startPos:ln], byte('\n')))
	}
	return ln, nil
}

func (l *Logger) write(data []byte) (n int, err error) {
	hour, min, sec := time.Now().Clock()
	if l.enableColor {
		return l.out.Write([]byte(fmt.Sprintf(
			"\x1b[%dm%02d:%02d:%02d %-10s |\x1b[0m %s",
			l.clr,
			hour, min, sec,
			l.sid, data,
		)))
	}
	return l.out.Write([]byte(fmt.Sprintf("%02d:%02d:%02d %-10s | %s", hour, min, sec, l.sid, data)))
}
