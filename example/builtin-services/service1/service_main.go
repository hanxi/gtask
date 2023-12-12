package service1

import (
	"context"
	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/log"
)

func init() {
	gtask.InitService("service1", NewMyBuiltinService)
}

type MyBuiltinService struct {
	*gtask.BaseService
}

func NewMyBuiltinService(ctx context.Context) gtask.Service {
	baseService := gtask.NewBaseService(ctx)
	s := &MyBuiltinService{
		BaseService: baseService,
	}
	s.Register("rpcPing", s.rpcPing)
	return s
}

func (s *MyBuiltinService) rpcPing(arg interface{}) interface{} {
	log.Info("in MyBuiltinService rpcPing")
	return "pong"
}
