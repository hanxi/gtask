package main

import (
	"time"

	"github.com/hanxi/gtask"
)

func main() {
	gtask.Init("config.json")

	go func() {
		time.Sleep(3 * time.Second)
		gtask.Stop()
	}()

	gtask.Run()
}
