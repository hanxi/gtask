package main

import (
	"context"
	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/log"
)

func init() {
	gtask.InitService("myplugin2", NewMyPluginService)
}

type MyPluginService struct {
	*gtask.BaseService
}

func NewMyPluginService(ctx context.Context) gtask.Service {
	baseService := gtask.NewBaseService(ctx)
	s := &MyPluginService{
		BaseService: baseService,
	}
	s.Register("rpcPing2", s.rpcPing2)
	return s
}

func (s *MyPluginService) rpcPing2(arg interface{}) interface{} {
	log.Info("in MyPluginService rpcPing2", "arg", arg)
	return "pong2"
}
