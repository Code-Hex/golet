package golet

import (
	"fmt"
	"io"
	"os"
)

// Context struct for golet
type Context struct {
	w    io.Writer
	port int
}

// Close closes the Pipe writer, rendering it unusable for I/O.
// It returns an error, if any.
func (c *Context) Close() error {
	return c.w.(*os.File).Close()
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
	return io.Copy(c.w, src)
}

// Improved io.writer for golet
func (c *Context) Write(p []byte) (n int, err error) {
	return c.w.Write(p)
}

// Println formats using the default formats for its operands and writes to golet writer.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.
func (c *Context) Println(a ...interface{}) (n int, err error) {
	return fmt.Fprintln(c.w, a...)
}

// Print formats using the default formats for its operands and writes to golet writer.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
func (c *Context) Print(a ...interface{}) (n int, err error) {
	return fmt.Fprint(c.w, a...)
}

// Printf formats according to a format specifier and writes to golet writer
// It returns the number of bytes written and any write error encountered.
func (c *Context) Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(c.w, format, a...)
}
