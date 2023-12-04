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

type SchedulerService struct {
	*BaseService
	services      map[uint64]Service
	wg            sync.WaitGroup
	allMessageOut chan Message
}

func NewSchedulerService(ctx context.Context) *SchedulerService {
	service := NewBaseServiceNoId(ctx)
	service.id = SERVICE_ID_SCHEDULER
	s := &SchedulerService{
		BaseService:   service,
		services:      make(map[uint64]Service),
		allMessageOut: make(chan Message, config.C.MsgQueueLen),
	}
	s.Register("registerService", registerService)
	return s
}

func (s *SchedulerService) wait() {
	// 监听系统信号
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signals:
		// 收到信号，取消 context
		log.Info("Scheduler received an interrupt signal, stopping services...")
		s.cancel()
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

func (s *SchedulerService) RegisterService(service Service) error {
	ret, err := s.Call(SERVICE_ID_SCHEDULER, &Content{Name: "registerService", Arg: &registerServiceArg{s: s, service: service}})
	if err != nil {
		return err
	}
	if ret != nil {
		return ret.(error)
	}
	return nil
}

type registerServiceArg struct {
	s       *SchedulerService
	service Service
}

func registerService(arg interface{}) interface{} {
	s := arg.(*registerServiceArg).s
	service := arg.(*registerServiceArg).service
	return s.registerService(service)
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
			service.stop()
		}
		log.Info("stop", "id", service.GetID())
		delete(s.services, id)
	}
	s.wg.Wait()
	s.BaseService.stop()
}
