package gtask

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

type Message struct {
	From    uint32
	To      uint32
	Content interface{}
}

type Content struct {
	Name string
	Args interface{}
}

// 定义 Service 接口
type Service interface {
	Dispatch(wg *sync.WaitGroup)
	Stop()
	GetId() uint32
	Send(to uint32, content interface{}) error
	Handler(name string, fn HandlerFunc)

	sendMessage(msg Message) error
	setId(id uint32)
	getStatus() ServiceStatus
	setStatus(status ServiceStatus)
}

type ServiceStatus int

const (
	SERVICE_STATUS_CREATE ServiceStatus = iota
	SERVICE_STATUS_INIT
	SERVICE_STATUS_RUNNING
	SERVICE_STATUS_DIE
)

type HandlerFunc func(args interface{}) interface{}

type BaseService struct {
	id        uint32
	chanMsg   chan Message
	ctx       context.Context
	cancel    context.CancelFunc
	status    ServiceStatus
	scheduler *Scheduler
	handlers  map[string]HandlerFunc
}

func NewBaseService(ctx context.Context, scheduler *Scheduler, msgSize uint32) *BaseService {
	ctx, cancel := context.WithCancel(ctx)
	return &BaseService{
		id:        uint32(0),
		chanMsg:   make(chan Message, msgSize),
		ctx:       ctx,
		cancel:    cancel,
		status:    SERVICE_STATUS_CREATE,
		scheduler: scheduler,
		handlers:  make(map[string]HandlerFunc),
	}
}

func (s *BaseService) Handler(name string, fn HandlerFunc) {
	s.handlers[name] = fn
}

func (s *BaseService) Stop() {
	s.setStatus(SERVICE_STATUS_DIE)
	s.cancel()
}

func (s *BaseService) Dispatch(wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	s.setStatus(SERVICE_STATUS_RUNNING)
	for {
		select {
		case msg := <-s.chanMsg:
			fmt.Printf("Service %d received a message from %d: %+v\n", s.GetId(), msg.From, msg.Content)
			content, ok := msg.Content.(Content)
			if ok {
				handFunc, exist := s.handlers[content.Name]
				if exist {
					ret := handFunc(content.Args)
					fmt.Printf("Service %d handler:%s ret:%+v\n", s.GetId(), content.Name, ret)
					// TODO: 把 ret 发送回去 还需要 session
				}
			} else {
				fmt.Println("unknow content")
			}
		case <-s.ctx.Done():
			fmt.Println("Service is closing")
			return
		}
	}
}

func (s *BaseService) GetId() uint32 {
	return s.id
}

func (s *BaseService) setId(id uint32) {
	s.id = id
}

func (s *BaseService) sendMessage(msg Message) error {
	select {
	case s.chanMsg <- msg:
		return nil
	default:
		return fmt.Errorf("message queue is full")
	}
}

func (s *BaseService) getStatus() ServiceStatus {
	return s.status
}

func (s *BaseService) setStatus(status ServiceStatus) {
	s.status = status
}

func (s *BaseService) Send(to uint32, content interface{}) error {
	return s.scheduler.Send(s.id, to, content)
}

type Scheduler struct {
	nextId   uint32
	services sync.Map
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) RegisterService(service Service) uint32 {
	id := atomic.AddUint32(&s.nextId, 1)
	_, exist := s.services.Load(id)
	if exist {
		fmt.Errorf("Scheduler RegisterService failed. id:%d", id)
		return 0
	}
	service.setId(id) // 设置服务的 ID
	s.services.Store(id, service)
	service.setStatus(SERVICE_STATUS_INIT)
	return id
}

func (s *Scheduler) Stop() {
	s.services.Range(func(key, value interface{}) bool {
		id := key.(uint32)
		service := value.(Service)
		service.Stop()
		s.services.Delete(id)
		return true
	})

	s.wg.Wait()
	s.cancel() // 发出关闭调度器的信号
}

func (s *Scheduler) DispatchAll() {
	s.services.Range(func(key, value interface{}) bool {
		service := value.(Service)
		s.Dispatch(service)
		return true
	})
}

func (s *Scheduler) Dispatch(service Service) error {
	if service.getStatus() != SERVICE_STATUS_INIT {
		return fmt.Errorf("Scheduler RegisterService failed. id:%d", service.GetId())
	}
	go service.Dispatch(&s.wg)
	return nil
}

func (s *Scheduler) Loop() {
	s.DispatchAll()

	// 监听系统信号
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	select {
	case <-signals:
		// 收到信号，取消 context
		fmt.Println("Scheduler received an interrupt signal, stopping services...")
		s.cancel()
	case <-s.ctx.Done():
		// Context 被取消，退出
		fmt.Println("Scheduler is shutting down...")
	}

	s.Stop()
}

func (s *Scheduler) Send(from, to uint32, content interface{}) error {
	val, ok := s.services.Load(to)
	if !ok {
		return fmt.Errorf("service with id %d does not exist", to)
	}

	service := val.(Service)
	msg := Message{From: from, To: to, Content: content}
	if err := service.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send message to service %d: %v", to, err)
	}
	return nil
}

// PluginService 是一个实现了 Service 接口的插件服务
type PluginService struct {
	*BaseService
}

func NewPluginService(ctx context.Context, scheduler *Scheduler, msgSize uint32) *PluginService {
	baseService := NewBaseService(ctx, scheduler, msgSize)
	return &PluginService{
		BaseService: baseService,
	}
}

// Run 重写了 BaseService 的 Run 方法，以提供特定的执行逻辑
func (p *PluginService) Dispatch(wg *sync.WaitGroup) {
	fmt.Printf("PluginService %d is running\n", p.GetId())
	p.BaseService.Dispatch(wg) // 调用基类的 Run 方法执行基本的消息处理逻辑
}

// Stop 重写了 BaseService 的 Stop 方法，以提供特定的停止逻辑
func (p *PluginService) Stop() {
	fmt.Printf("PluginService %d is stopping\n", p.GetId())
	p.BaseService.Stop() // 调用基类的 Stop 方法来执行取消操作
}
