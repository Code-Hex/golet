package main

import (
	"time"

	"github.com/Code-Hex/golet"
)

func main() {
	p := golet.New()
	p.EnableColor()
	p.SetInterval(time.Second * 0)

	p.Add(
		golet.Service{
			Exec:   "ping google.com",
			Worker: 2,
		},
		golet.Service{
			Exec: "ping ie.u-ryukyu.ac.jp",
			Tag:  "ryukyu",
		},
		golet.Service{
			Exec:   "ping yahoo.co.jp",
			Tag:    "yahoo",
			Worker: 3,
		},
	)

	p.Add(golet.Service{
		Exec:   "ping gigazine.net",
		Tag:    "gigazine",
		Worker: 1,
	})

	p.Run()
}
