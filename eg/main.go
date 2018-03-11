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
	cctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-time.After(time.Second * 10)
		cancel()
	}()

	p := golet.New(cctx)
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
		Code:   serveCode(),
		Worker: 3,
	})

	p.Run()
}

func serveCode() func(context.Context) error {
	return func(ctx context.Context) error {
		c := ctx.(*golet.Context)
		c.Println("Hello golet!! Port:", c.Port())
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello, World")
			buf := strings.NewReader("This is log string\nNew line1\nNew line2\nNew line3\n")
			c.Copy(buf)
		})
		panicChan := make(chan interface{}, 1)
		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			if err := http.ListenAndServe(c.ServePort(), mux); err != nil {
				panic(err)
			}
		}()
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
				c.Println("context cancel")
				return nil
			case p := <-panicChan:
				return fmt.Errorf("%#v", p)
			}
		}
	}
}
