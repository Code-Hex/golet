Golet
=====
[![Build Status](https://travis-ci.org/Code-Hex/golet.svg?branch=master)](https://travis-ci.org/Code-Hex/golet) [![GoDoc](https://godoc.org/github.com/Code-Hex/golet?status.svg)](https://godoc.org/github.com/Code-Hex/golet) [![Go Report Card](https://goreportcard.com/badge/github.com/Code-Hex/golet)](https://goreportcard.com/report/github.com/Code-Hex/golet)  
Golet can manage many services with goroutine from one golang program.  
It supports go version 1.7 or higher.  
It is based on the idea of [Proclet](https://metacpan.org/pod/Proclet).  
Proclet is a great module in Perl.

# Synopsis

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "syscall"
    "time"

    "github.com/Code-Hex/golet"
)

func main() {
    p := golet.New(context.Background())
    p.EnableColor()
    // Execution interval
    p.SetInterval(time.Second * 1)

    p.Env(map[string]string{
        "NAME":  "codehex",
        "VALUE": "121",
    })

    p.Add(
        golet.Service{
            Exec: "plackup --port $PORT",
            Tag:  "plack",
        },
        golet.Service{
            Exec:   "echo 'This is cron!!'",
            Every:  "30 * * * * *",
            Worker: 2,
            Tag:    "cron",
        },
    )

    p.Add(golet.Service{
        Code: func(ctx context.Context) error {
            c := ctx.(*golet.Context)
            c.Println("Hello golet!! Port:", c.Port())
            mux := http.NewServeMux()
            mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                fmt.Fprintf(w, "Hello, World")
                buf := strings.NewReader("This is log string\nNew line1\nNew line2\nNew line3")
                c.Copy(buf)
            })
            go http.ListenAndServe(c.ServePort(), mux)
            for {
                select {
                // You can notify signal received.
                case <-c.Recv():
                    signal, err := c.Signal()
                    if err != nil {
                        c.Println(err.Error())
                        return err
                    }
                    switch signal {
                    case syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT:
                        c.Println(signal.String())
                        return nil
                    }
                case <-ctx.Done():
                    return nil
                }
            }
        },
        Worker: 3,
    })

    p.Run()
}
```
See [eg](https://github.com/Code-Hex/golet/tree/master/eg).
# Logging
In case to run code of synopsis.
![Logging](https://user-images.githubusercontent.com/6500104/37191403-ac145616-23a2-11e8-9edd-d54175450b84.gif)

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
        // Replace $PORT and automatically assigned port number.
        Exec: "plackup --port $PORT",
        Tag:  "plack", // Keyword for log.
    },
    golet.Service{
        Exec:   "echo 'This is cron!!'",
        // Crontab like format. 
        Every:  "30 * * * * *", // See https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format
        Worker: 2,              // Number of goroutine. The maximum number of workers is 100.
        Tag:    "cron",
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

and, If you execute service as command, you can get also `PORT` env variable.

## golet.Context

See, https://godoc.org/github.com/Code-Hex/golet#Context

`golet.Context` can be to `context.Context`, `io.Writer` also `io.Closer`.

So, you can try to write strange code like this.

```go
golet.Service{
    Code: func(ctx context.Context) error {
        // die program
        c := ctx.(*golet.Context)
        fmt.Fprintf(c, "Hello, World!!\n")
        c.Printf("Hello, Port: %d\n", c.Port())
        time.Sleep(time.Second * 1)
        select {
        case <-c.Recv():
            signal, err := c.Signal()
            if err != nil {
                c.Println(err.Error())
                return err
            }
            switch signal {
            case syscall.SIGTERM, syscall.SIGHUP:
                c.Println(signal.String())
                return fmt.Errorf("die message")
            case syscall.SIGINT:
                return nil
            }
            return nil
        }
    },
}
```

# Installation

    go get -u github.com/Code-Hex/golet

# Contribution

1. Fork [https://github.com/Code-Hex/golet/fork](https://github.com/Code-Hex/golet/fork)
2. Commit your changes
3. Create a new Pull Request

I'm waiting for a lot of PR.

# Future

- [ ] Create like [foreman](https://github.com/ddollar/foreman) command
- [ ] Support better windows
- [ ] Write a test for signals for some distribution

# Author

[codehex](https://twitter.com/CodeHex)