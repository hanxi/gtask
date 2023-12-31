package gtask

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask/config"
	"github.com/hanxi/gtask/log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	SERVICE_ID_SCHEDULER uint64 = 1 // 调度服务
)

type SchedulerService struct {
	*BaseService
	services      map[uint64]Service
	wg            sync.WaitGroup
	allMessageOut chan Message
}

func newSchedulerService(ctx context.Context) *SchedulerService {
	service := NewBaseServiceWithId(ctx, SERVICE_ID_SCHEDULER)
	s := &SchedulerService{
		BaseService:   service,
		services:      make(map[uint64]Service),
		allMessageOut: make(chan Message, config.C.MsgQueueLen),
	}
	s.Register("rpcRegisterService", s.rpcRegisterService)
	s.Register("rpcSpawnService", s.rpcSpawnService)
	s.Register("rpcStop", s.rpcStop)
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

func (s *SchedulerService) run(c Service, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("in run", "service", s, "c", c)
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
	log.Info("end run", "id", s.GetID())
}

func (s *SchedulerService) registerService(service Service) error {
	id := service.GetID()
	if service.GetStatus() != SERVICE_STATUS_CREATE {
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
	go service.run(service, &s.wg)
	return nil
}

// SingleFlight 是一个结构体，用于确保给定函数的单个执行
type SingleFlight struct {
	mu sync.Mutex
}

// Do 方法接收一个将被执行的函数，确保在同一时刻只有一个 goroutine 可以执行
func (sf *SingleFlight) Do(f func()) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	f()
}

var sf SingleFlight

func (s *SchedulerService) stop() {
	sf.Do(func() {
		log.Info("stop", "id", s.GetID())
		for id, service := range s.services {
			if id != SERVICE_ID_SCHEDULER {
				// 发消息的方式关闭服务
				_, err := s.Call(id, Content{Name: "rpcStop", Arg: service})
				if err != nil {
					log.Error("stop call failed.", "id", id, "err", err)
				}
			}
			log.Info("stop", "id", service.GetID())
			delete(s.services, id)
		}
		log.Info("stop ok", "id", s.GetID())
		s.BaseService.stop()
		s.wg.Wait()
	})
}

func (s *SchedulerService) rpcRegisterService(arg interface{}) interface{} {
	service := arg.(Service)
	return s.registerService(service)
}

// 服务创建函数
type newServiceFunc func(ctx context.Context) Service

var newServiceFuncs = map[string]newServiceFunc{}

// 初始化服务的创建函数，在服务的 init 函数内执行
func InitService(name string, fn newServiceFunc) {
	newServiceFuncs[name] = fn
}

// 只在 launcher 服务内执行
func getServiceNewFunc(name string) newServiceFunc {
	return newServiceFuncs[name]
}

type SpawnServiceRet struct {
	ID  uint64
	Err error
}

func (s *SchedulerService) rpcSpawnService(arg interface{}) interface{} {
	name := arg.(string)
	newServiceFunc := getServiceNewFunc(name)
	if newServiceFunc == nil {
		err := fmt.Errorf("service %s not in newServiceFuncs", name)
		return &SpawnServiceRet{ID: 0, Err: err}
	}

	service := newServiceFunc(context.Background()) // 调用函数获取Service实例
	err := s.registerService(service)
	if err != nil {
		return &SpawnServiceRet{ID: 0, Err: err}
	}
	return &SpawnServiceRet{ID: service.GetID(), Err: nil}
}

func (s *SchedulerService) rpcStop(arg interface{}) interface{} {
	log.Info("rpcStop", "id", s.GetID())
	// 目前协议过来的只能通过新的 goroutine 来调用,否则会卡住无法分发全局消息
	// TODO: 支持一个服务有多个 goroutine 且一个服务同时只有一个 goroutine 在运行
	go s.stop()
	return nil
}
