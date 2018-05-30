package golet

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Context struct for golet
type Context struct {
	ctx    *signalCtx
	logger *Logger
	port   int
}

// Port returns assgined port
func (c *Context) Port() int {
	return c.port
}

// ServePort returns assigned port which is serve format
func (c *Context) ServePort() string {
	return fmt.Sprintf(":%d", c.port)
}

// Copy method is wrapped by io.Copy
func (c *Context) Copy(src io.Reader) (written int64, err error) {
	return io.Copy(c.logger, src)
}

// Improved io.writer for golet
func (c *Context) Write(p []byte) (n int, err error) {
	return c.logger.Write(p)
}

// Println formats using the default formats for its operands and writes to golet writer.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.
func (c *Context) Println(a ...interface{}) (n int, err error) {
	return fmt.Fprintln(c.logger, a...)
}

// Print formats using the default formats for its operands and writes to golet writer.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
func (c *Context) Print(a ...interface{}) (n int, err error) {
	return fmt.Fprint(c.logger, a...)
}

// Printf formats according to a format specifier and writes to golet writer
// It returns the number of bytes written and any write error encountered.
func (c *Context) Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(c.logger, format, a...)
}

// Recv send channel when a process receives a signal.
func (c *Context) Recv() <-chan struct{} {
	return c.ctx.Recv()
}

// Signal returns os.Signal and error.
func (c *Context) Signal() (os.Signal, error) {
	return c.ctx.Signal()
}

/* They are methods for context.Context */

// Deadline is implemented for context.Context
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

// Done is implemented for context.Context
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err is implemented for context.Context
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value is implemented for context.Context
func (c *Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}
