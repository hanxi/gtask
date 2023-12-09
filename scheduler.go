package gtask

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask/config"
	"github.com/hanxi/gtask/log"
	"os"
	"os/signal"
	"plugin"
	"sync"
	"syscall"
)

type SchedulerService struct {
	*BaseService
	services      map[uint64]Service
	wg            sync.WaitGroup
	allMessageOut chan Message
}

func newSchedulerService(ctx context.Context) *SchedulerService {
	service := NewBaseServiceNoId(ctx)
	service.id = SERVICE_ID_SCHEDULER
	s := &SchedulerService{
		BaseService:   service,
		services:      make(map[uint64]Service),
		allMessageOut: make(chan Message, config.C.MsgQueueLen),
	}
	s.Register("rpcRegisterService", s.rpcRegisterService)
	s.Register("rpcNewServiceFromPlugin", s.rpcNewServiceFromPlugin)
	return s
}

// 主 goroutine 里运行
func (s *SchedulerService) wait() {
	// 监听系统信号
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signals:
		// 收到信号，取消 context
		log.Info("Scheduler received an interrupt signal, stopping services...")
	case <-s.ctx.Done():
		log.Info("Service is closing in wait", "id", s.GetID())
	}
	s.stop()
}

func (s *SchedulerService) run(wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("in run", "service", s)
	s.setStatus(SERVICE_STATUS_RUNNING)

	for {
		select {
		case msg := <-s.getMessageIn():
			s.exec(&msg)
		case msg := <-s.allMessageOut:
			if service, ok := s.services[msg.To]; ok {
				service.getMessageIn() <- msg
			} else {
				log.Error("No service found", "ID", msg.To)
			}
		case <-s.ctx.Done():
			log.Info("Service is closing", "id", s.GetID())
			return
		}
	}
}

func (s *SchedulerService) registerService(service Service) error {
	id := service.GetID()
	if service.getStatus() != SERVICE_STATUS_CREATE {
		log.Error("Service already register.", "id", service.GetID())
		return fmt.Errorf("Service already register. id:%d", id)
	}

	_, exist := s.services[id]
	if exist {
		log.Error("registerService id already exist.", "id", id)
		return fmt.Errorf("Service with ID %d already exists", id)
	}

	s.services[id] = service
	service.setStatus(SERVICE_STATUS_INIT)
	service.setMessageOut(s.allMessageOut)
	s.wg.Add(1)
	go service.run(&s.wg)
	return nil
}

func (s *SchedulerService) stop() {
	for id, service := range s.services {
		if id != SERVICE_ID_SCHEDULER {
			// 发消息的方式关闭服务
			s.Call(id, &Content{Name: "rpcStop"})
		}
		log.Info("stop", "id", service.GetID())
		delete(s.services, id)
	}
	s.BaseService.stop()
	s.wg.Wait()
}

func (s *SchedulerService) rpcRegisterService(arg interface{}) interface{} {
	service := arg.(Service)
	return s.registerService(service)
}

func (s *SchedulerService) rpcNewServiceFromPlugin(arg interface{}) interface{} {
	serviceFile := arg.(string)
	id, err := s.newServiceFromPlugin(serviceFile)
	return &NewServiceFromPluginRet{ID: id, Err: err}
}

type NewServiceFromPluginRet struct {
	ID  uint64
	Err error
}

// 从插件中开服务
func (s *SchedulerService) newServiceFromPlugin(serviceFile string) (uint64, error) {
	p, err := plugin.Open(serviceFile + ".so") // 打开插件文件
	if err != nil {
		return 0, err
	}

	newFunc, err := p.Lookup("NewService") // 查找插件导出的NewService函数
	if err != nil {
		return 0, err
	}

	newServiceFunc, ok := newFunc.(func(ctx context.Context) Service) // 类型断言为正确的函数签名
	if !ok {
		return 0, fmt.Errorf("Plugin %s has no 'NewService' function of the correct type", serviceFile)
	}

	service := newServiceFunc(context.Background()) // 调用函数获取Service实例
	s.registerService(service)
	return service.GetID(), nil
}
