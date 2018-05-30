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
	Code   func(context.Context) error // Routine of services.
	Worker int                         // Number of goroutine. The maximum number of workers is 100.
	Tag    string                      // Keyword for log.
	Every  string                      // Crontab like format. See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format

	id     string
	ctx    *Context // This can be io.Writer. see context.go
	logger *Logger
}

func (s *Service) createContext(ctx *signalCtx, logger *Logger, port int) error {
	s.ctx = &Context{
		ctx:    ctx,
		logger: logger,
		port:   port,
	}
	return nil
}

// Create a command
func (s *Service) prepare() *exec.Cmd {
	c := strings.Replace(s.Exec, "$PORT", fmt.Sprintf("%d", s.ctx.Port()), -1)
	args := append(shell, c)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = s.ctx
	cmd.Stderr = s.ctx
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", s.ctx.Port()))
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
