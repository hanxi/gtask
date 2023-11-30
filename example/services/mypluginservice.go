package main

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask"
)

type MyPluginService struct {
	*gtask.BaseService
}

func New(ctx context.Context, scheduler *gtask.Scheduler, msgSize uint32) gtask.Service {
	baseService := gtask.NewBaseService(ctx, scheduler, msgSize)
	service := &MyPluginService{
		BaseService: baseService.(*gtask.BaseService),
	}
	service.Handler("hello", func(args interface{}) interface{} {
		fmt.Println("3in hello handler")
		return "hello world 3"
	})
	service.Handler("world", func(args interface{}) interface{} {
		fmt.Println("3in world handler")
		return "hello world3"
	})
	return service
}
