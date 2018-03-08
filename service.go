package golet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Service struct to add services to golet.
type Service struct {
	Exec   string
	Code   func(context.Context, *Context) // Routine of services.
	Worker int                             // Number of goroutine. The maximum number of workers is 100.
	Tag    string                          // Keyword for log.
	Every  string                          // Crontab like format. See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format

	color   color
	reader  *os.File
	ctx     *Context // This can be io.Writer. see context.go
	tmpPort int
}

func (s *Service) createContext(i int) error {
	in, out, err := os.Pipe()
	if err != nil {
		return err
	}
	s.reader = in
	s.ctx = &Context{
		w:    out,
		port: s.tmpPort + i,
	}
	return nil
}

// Create a command
func (s *Service) prepare() *exec.Cmd {
	c := strings.Replace(s.Exec, "$PORT", fmt.Sprintf("%d", s.tmpPort), -1)
	args := append(shell, c)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = s.ctx
	cmd.Stderr = s.ctx
	return cmd
}

func (s *Service) isExecute() bool {
	return s.Code == nil && s.Exec != ""
}

func (s *Service) isCode() bool {
	return s.Code != nil
}

func (s *Service) isCron() bool {
	return s.Every != ""
}

// Printf formats according to a format specifier and writes to golet service
// It returns the number of bytes written and any write error encountered.
func (s *Service) Printf(format string, a ...interface{}) (n int, err error) {
	return s.ctx.Printf(format, a...)
}
