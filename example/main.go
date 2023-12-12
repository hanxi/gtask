package main

import (
	"context"
	"time"

	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/example/builtin-services/service1"
	"github.com/hanxi/gtask/log"
)

func main() {
	gtask.Init("config.json")

	go func() {
		id, err := gtask.SpawnService("service1")
		if err != nil {
			log.Error("SpawnService service1 filed", "err", err)
		}
		log.Info("service1", "id", id)
	}()

	go func() {
		s1 := service1.NewMyBuiltinService(context.Background())
		err := gtask.RegisterService(s1)
		if err != nil {
			log.Error("NewMyBuiltinService service1 filed", "err", err)
		}
		log.Info("service1", "id", s1.GetID())
	}()

	go func() {
		id, err := gtask.NewServiceFromPlugin("myplugin")
		if err != nil {
			log.Error("NewServiceFromPlugin myplugin filed", "err", err)
		}
		log.Info("myplugin", "id", id)
	}()

	go func() {
		id, err := gtask.NewServiceFromPlugin("myplugin2")
		if err != nil {
			log.Error("NewServiceFromPlugin myplugin2 filed", "err", err)
		}
		log.Info("myplugin2", "id", id)
	}()

	go func() {
		time.Sleep(3 * time.Second)
		gtask.Stop()
	}()

	gtask.Run()
}
