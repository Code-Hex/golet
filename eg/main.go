package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
		Code: func(ctx context.Context, w io.Writer, port int) {
			fmt.Fprintln(w, "Hello golet!! Port:", port)
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "Hello, World")
			})

			go http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
			<-ctx.Done()
		},
		Worker: 3,
	})

	p.Run()
}
