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

	// Send TERM signal when context cancel
	p.SetCtxCancelSignal(syscall.SIGTERM)

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
		golet.Service{
			Exec: `perl -E 'sleep 2; say "THIS IS DIE PROGRAM"; die;'`,
			Tag:  "DIE-Perl",
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
		srv := &http.Server{Addr: c.ServePort(), Handler: mux}

		// for ListenAndServe's error
		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			if err := srv.ListenAndServe(); err != nil {
				panic(err)
			}
		}()

		// Wait channels
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
				// If you send TERM or HUP, restart this callback immediately.
				case syscall.SIGTERM, syscall.SIGHUP:
					c.Println(signal.String())
					if err := srv.Shutdown(ctx); err != nil {
						return err
					}
					return fmt.Errorf("signal recieved SIGTERM, SIGHUP as error")
				// End of run
				case syscall.SIGINT:
					c.Println(signal.String())
					return nil
				}
			// To catch context cancel
			case <-ctx.Done():
				c.Println("context cancel")
				return srv.Shutdown(ctx)
			// panic for ListenAndServe
			case p := <-panicChan:
				return p.(error)
			}
		}
	}
}
