golet
=====
I made golet based on the idea of [Proclet](https://metacpan.org/pod/Proclet).  
Proclet is great module in Perl.  

# Synopsis
```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Code-Hex/golet"
)

func main() {
	p := golet.New(context.Background())
	p.EnableColor()
	// Execcution interval
	p.SetInterval(time.Second * 1)

	p.Env(map[string]string{
		"NAME":  "codehex",
		"VALUE": "121",
	})

	p.Add(
		golet.Service{
			Exec: "ping google.com",
			Tag:  "ping",
		},
		golet.Service{
			Exec:   "echo 'Worker is 2!! PORT: $PORT'",
			Every:  "30 * * * * *",
			Worker: 2,
			Tag:    "echo",
		},
	)

	p.Add(golet.Service{
		Code: func(w io.Writer, port int) error {
			fmt.Fprintln(w, "Hello golet!! Port:", port)
			fmt.Fprintln(w, os.Getenv("NAME"), os.Getenv("VALUE"))
			return nil
		},
		Worker: 3,
		Every:  "@every 10s",
	})

	p.Run()
}
```

# Description
Golet is minimalistic Supervisor like Proclet, fork and manage many services from one golang program.

# Logging
In case to run code of synopsis.
![Logging](https://cloud.githubusercontent.com/assets/6500104/22722641/efd64978-edfb-11e6-8f21-3d44e0ea7f52.png)

# Usage
## Basic
Make sure to generate a struct from the `New(context.Context)` function.  
```go
p := golet.New(context.Background())
```
Next, We can add the service using the `Service` struct.  
`Add(services ...Service) error` method is also possible to pass multiple `Service` struct arguments.
```go
p.Add(
	golet.Service{
		Exec: "ping google.com",
		Tag:  "ping",  // Keyword for log.
	},
	golet.Service{
		// Replace $PORT and automatically assigned port number.
		Exec:   "echo 'Worker is 2!! PORT: $PORT'",
		// Crontab like format. 
		Every:  "30 * * * * *", // See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format
		Worker: 2,              // Number of goroutine. The maximum number of workers is 100.
		Tag:    "echo",
	},
)
```
Finally, You can run many services. use `Run() error`
```go
p.Run()
```

## Option
The default option is like this.
```
interval:   0
color:      false
logger:     colorable.NewColorableStderr()
logWorker:  true
execNotice: true
```
By using the [go-colorable](https://github.com/mattn/go-colorable), colored output is also compatible with windows.  
You can change these options by using the following method.
```go
// SetInterval can specify the interval at which the command is executed.
func (c *config) SetInterval(t time.Duration) { c.interval = t }

// EnableColor can output colored log.
func (c *config) EnableColor() { c.color = true }

// SetLogger can specify the *os.File
// for example in https://github.com/lestrrat/go-file-rotatelogs
/*
      logf, _ := rotatelogs.New(
  	    "/path/to/access_log.%Y%m%d%H%M",
  	    rotatelogs.WithLinkName("/path/to/access_log"),
  	    rotatelogs.WithMaxAge(24 * time.Hour),
  	    rotatelogs.WithRotationTime(time.Hour),
      )

	  golet.New(context.Background()).SetLogger(logf)
*/
func (c *config) SetLogger(f io.Writer) { c.logger = f }

// DisableLogger is prevent to output log
func (c *config) DisableLogger() { c.logWorker = false }

// DisableExecNotice is disable execute notifications
func (c *config) DisableExecNotice() { c.execNotice = false }
```
## Environment variables
You can use temporary environment variables in golet program.
```go
p.Env(map[string]string{
	"NAME":  "codehex",
	"VALUE": "121",
})
```
# Installation

    go get github.com/Code-Hex/golet

# Contribution
1. Fork [https://github.com/Code-Hex/golet/fork](https://github.com/Code-Hex/golet/fork)
2. Commit your changes
3. Create a new Pull Request

I'm waiting for a lot of PR.