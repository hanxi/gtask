package main

import (
	"context"
	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/log"
)

type MyPluginService struct {
	*gtask.BaseService
}

func New(ctx context.Context, scheduler *gtask.Scheduler) gtask.Service {
	baseService := gtask.NewBaseService(ctx, scheduler)
	service := &MyPluginService{
		BaseService: baseService.(*gtask.BaseService),
	}
	service.Register("hello", func(args interface{}) interface{} {
		log.Info("3in hello handler")
		return "hello world 3"
	})
	service.Register("world", func(args interface{}) interface{} {
		log.Info("3in world handler")

		ret, err := service.Call(2, &gtask.Content{Name: "hello", Args: "hellocallarg"})
		if err != nil {
			log.Error("call failed.", "err", err)
		}
		log.Info("debug4", "ret", ret)

		return "hello world3"
	})
	return service
}
