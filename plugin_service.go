package gtask

import (
	"context"

	"github.com/hanxi/gtask/log"
)

// PluginService 是一个实现了 Service 接口的插件服务
type PluginService struct {
	*BaseService
}

func NewPluginService(ctx context.Context) Service {
	service := NewBaseService(ctx)
	return &PluginService{
		BaseService: service,
	}
}

// Stop 重写了 BaseService 的 stop 方法，以提供特定的停止逻辑
func (p *PluginService) stop() {
	log.Info("PluginService is stopping", "id", p.GetID())
	p.BaseService.stop() // 调用基类的 Stop 方法来执行取消操作
}
