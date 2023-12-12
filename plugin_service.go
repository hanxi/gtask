package gtask

import (
	"context"
	"github.com/hanxi/gtask/log"
	"plugin"
)

const (
	SERVICE_ID_PLUGIN uint64 = 2 // 插件服务
)

// PluginService 是一个实现了 Service 接口的插件服务
type PluginService struct {
	*BaseService
}

func NewPluginService(ctx context.Context) Service {
	service := NewBaseServiceWithId(ctx, SERVICE_ID_PLUGIN)
	s := &PluginService{
		BaseService: service,
	}
	s.Register("rpcNewServiceFromPlugin", s.rpcNewServiceFromPlugin)
	log.Debug("newServiceFromPlugin", "id", service.GetID())
	return s
}

// 从插件中开服务
func (s *PluginService) newServiceFromPlugin(name string) (uint64, error) {
	_, err := plugin.Open(name + ".so") // 打开插件文件
	if err != nil {
		return 0, err
	}

	return SpawnService(name)
}

type NewServiceFromPluginRet struct {
	ID  uint64
	Err error
}

func (s *PluginService) rpcNewServiceFromPlugin(arg interface{}) interface{} {
	name := arg.(string)
	id, err := s.newServiceFromPlugin(name)
	return &NewServiceFromPluginRet{ID: id, Err: err}
}
