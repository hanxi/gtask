package main

import (
	"context"
	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/log"
)

func init() {
	// 名字和so文件名一致
	gtask.InitService("myplugin", NewMyPluginService)
}

type MyPluginService struct {
	*gtask.BaseService
}

func NewMyPluginService(ctx context.Context) gtask.Service {
	baseService := gtask.NewBaseService(ctx)
	s := &MyPluginService{
		BaseService: baseService,
	}
	s.Register("rpcPing", s.rpcPing)
	log.Info("NewMyPluginService", "id", s.GetID())
	return s
}

func (s *MyPluginService) rpcPing(arg interface{}) interface{} {
	log.Info("in MyPluginService rpcPing", "arg", arg)
	return "pong"
}

func (s *MyPluginService) OnInit() {
	log.Debug("MyPluginService OnInit", "id", s.GetID())
	err := s.Send(1025, gtask.Content{Name: "rpcPing", Arg: "ping"})
	if err != nil {
		log.Error("failed send", "err", err)
	}
}
