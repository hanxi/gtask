package gtask

import (
	"context"
	"github.com/hanxi/gtask/config"
)

// 查询
func QueryService() {

}

// 创建唯一服务
func UniqueService() {

}

var scheduler *SchedulerService

func Init(configPath string) {
	// 读取配置
	config.Load(configPath)

	// 加载基础服务
	scheduler = newSchedulerService(context.Background())
	scheduler.registerService(scheduler)

	// 加载插件服务
	pluginService := NewPluginService(context.Background())
	scheduler.registerService(pluginService)

	// 加载入口服务
	NewServiceFromPlugin(config.C.Bootstrap)
}

func Run() {
	scheduler.wait()
}

func Stop() {
	// 需要确保Call函数没并发问题
	scheduler.Call(SERVICE_ID_SCHEDULER, Content{Name: "rpcStop"})
}

// 关服务
func Kill(addr uint64) {
	// 需要确保Send函数没并发问题
	scheduler.Send(addr, Content{Name: "rpcStop"})
}

// 注册一个服务
func RegisterService(s Service) error {
	// TODO: 需要确保Call函数没并发问题
	ret, err := scheduler.Call(SERVICE_ID_SCHEDULER, Content{Name: "rpcRegisterService", Arg: s})
	if err != nil {
		return err
	}
	if ret != nil {
		return ret.(error)
	}
	return nil
}

// 开启一个服务
func SpawnService(name string) (uint64, error) {
	// TODO: 需要确保Call函数没并发问题
	ret, err := scheduler.Call(SERVICE_ID_SCHEDULER, Content{Name: "rpcSpawnService", Arg: name})
	if err != nil {
		return 0, err
	}
	sret := ret.(*SpawnServiceRet)
	return sret.ID, sret.Err
}

// 从插件加载服务
func NewServiceFromPlugin(name string) (uint64, error) {
	// TODO: 需要确保Call函数没并发问题
	ret, err := scheduler.Call(SERVICE_ID_PLUGIN, Content{Name: "rpcNewServiceFromPlugin", Arg: name})
	if err != nil {
		return 0, err
	}
	sret := ret.(*NewServiceFromPluginRet)
	return sret.ID, sret.Err
}
