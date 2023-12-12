package main

import (
	"context"
	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/log"
)

func init() {
	// 名字和so文件名一致
	gtask.InitService("bootstrap", NewBootstrapService)
}

type BootstrapService struct {
	*gtask.BaseService
}

func NewBootstrapService(ctx context.Context) gtask.Service {
	baseService := gtask.NewBaseService(ctx)
	s := &BootstrapService{
		BaseService: baseService,
	}
	log.Info("NewBootstrapService", "id", s.GetID())
	return s
}

func (s *BootstrapService) OnInit() {
	log.Debug("BootstrapService OnInit", "id", s.GetID())

	id1, err := gtask.SpawnService("service1")
	if err != nil {
		log.Error("SpawnService service1 filed", "err", err)
	}
	log.Info("service1", "id", id1)

	id2, err := gtask.NewServiceFromPlugin("myplugin")
	if err != nil {
		log.Error("NewServiceFromPlugin myplugin filed", "err", err)
	}
	log.Info("myplugin", "id", id2)

	id3, err := gtask.NewServiceFromPlugin("myplugin2")
	if err != nil {
		log.Error("NewServiceFromPlugin myplugin2 filed", "err", err)
	}
	log.Info("myplugin2", "id", id3)

	// TODO: OnInit 在实现单服务多goroutine前不能 Call
	err = s.Send(id1, gtask.Content{Name: "rpcPing", Arg: "ping"})
	if err != nil {
		log.Error("failed send", "err", err)
	}

	err = s.Send(id2, gtask.Content{Name: "rpcPing", Arg: "ping"})
	if err != nil {
		log.Error("failed send", "err", err)
	}

	err = s.Send(id3, gtask.Content{Name: "rpcPing2", Arg: "ping"})
	if err != nil {
		log.Error("failed send", "err", err)
	}
}
