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
				buf := strings.NewReader("This is log string\nNew line1\nNew line2\nNew line3\n")
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
