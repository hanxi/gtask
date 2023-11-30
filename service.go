package gtask

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask/config"
	"github.com/hanxi/gtask/log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
)

type Message struct {
	From    uint32
	To      uint32
	Content *Content
}

type Content struct {
	Name    string
	Args    interface{}
	ChanRet chan *RetInfo
	Session uint32
}

type RetInfo struct {
	// nil
	// interface{}
	ret interface{}
	err error
}

type HandlerFunc func(arg interface{}) interface{}

// 定义 Service 接口
type Service interface {
	Dispatch(wg *sync.WaitGroup)
	Stop()
	GetId() uint32
	Send(to uint32, content *Content) error                // 发送消息
	Call(to uint32, content *Content) (interface{}, error) // 阻塞rpc
	Register(name string, fn HandlerFunc)

	SendMessage(msg *Message) error
	SetId(id uint32)
	GetStatus() ServiceStatus
	SetStatus(status ServiceStatus)
}

type ServiceStatus int

const (
	SERVICE_STATUS_CREATE ServiceStatus = iota
	SERVICE_STATUS_INIT
	SERVICE_STATUS_RUNNING
	SERVICE_STATUS_DIE
)

type BaseService struct {
	id        uint32
	chanMsg   chan Message
	ctx       context.Context
	cancel    context.CancelFunc
	status    ServiceStatus
	scheduler *Scheduler
	handlers  map[string]HandlerFunc
}

func NewBaseService(ctx context.Context, scheduler *Scheduler) Service {
	ctx, cancel := context.WithCancel(ctx)
	return &BaseService{
		id:        uint32(0),
		chanMsg:   make(chan Message, config.C.MsgQueueLen),
		ctx:       ctx,
		cancel:    cancel,
		status:    SERVICE_STATUS_CREATE,
		scheduler: scheduler,
		handlers:  make(map[string]HandlerFunc),
	}
}

func (s *BaseService) Register(name string, fn HandlerFunc) {
	if _, ok := s.handlers[name]; ok {
		panic(fmt.Sprintf("function %s: already registered", name))
	}
	s.handlers[name] = fn
}

func (s *BaseService) Stop() {
	s.SetStatus(SERVICE_STATUS_DIE)
	s.cancel()
}

func (s *BaseService) ret(content *Content, ri *RetInfo) (err error) {
	if content.ChanRet == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	content.ChanRet <- ri
	return
}
func (s *BaseService) exec(content *Content) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if config.C.StackBufLen > 0 {
				buf := make([]byte, config.C.StackBufLen)
				l := runtime.Stack(buf, false)
				err = fmt.Errorf("%v: %s", r, buf[:l])
			} else {
				err = fmt.Errorf("%v", r)
			}

			s.ret(content, &RetInfo{err: fmt.Errorf("%v", r)})
		}
	}()

	// execute
	handFunc, exist := s.handlers[content.Name]
	if !exist {
		err = fmt.Errorf("unknow handler:%s", content.Name)
		s.ret(content, &RetInfo{err: err})
		return
	}

	ret := handFunc(content.Args)
	log.Info("Service handler ok", "id", s.GetId(), "handler", content.Name, "ret", ret)
	s.ret(content, &RetInfo{ret: ret})
	return
}

func (s *BaseService) Dispatch(wg *sync.WaitGroup) {
	defer wg.Done()
	s.SetStatus(SERVICE_STATUS_RUNNING)
	for {
		select {
		case msg := <-s.chanMsg:
			s.exec(msg.Content)
		case <-s.ctx.Done():
			log.Info("Service is closing")
			return
		}
	}
}

func (s *BaseService) GetId() uint32 {
	return s.id
}

func (s *BaseService) SetId(id uint32) {
	s.id = id
}

func (s *BaseService) SendMessage(msg *Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	select {
	case s.chanMsg <- *msg:
		return nil
	case <-s.ctx.Done():
		err = fmt.Errorf("failed to send message: service is stopping")
	default:
		err = fmt.Errorf("message queue is full")
	}
	return
}

func (s *BaseService) GetStatus() ServiceStatus {
	return s.status
}

func (s *BaseService) SetStatus(status ServiceStatus) {
	s.status = status
}

func (s *BaseService) Send(to uint32, content *Content) error {
	return s.scheduler.Send(s.id, to, content)
}

func (s *BaseService) Call(to uint32, content *Content) (interface{}, error) {
	return s.scheduler.Call(s.id, to, content)
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

func (s *Scheduler) RegisterService(service Service) (uint32, error) {
	id := atomic.AddUint32(&s.nextId, 1)
	_, exist := s.services.Load(id)
	if exist {
		return 0, fmt.Errorf("Service with ID %d already exists", id)
	}
	service.SetId(id) // 设置服务的 ID
	s.services.Store(id, service)
	service.SetStatus(SERVICE_STATUS_INIT)
	return id, nil
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
	if service.GetStatus() != SERVICE_STATUS_INIT {
		return fmt.Errorf("Scheduler RegisterService failed. id:%d", service.GetId())
	}
	s.wg.Add(1)
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
		log.Info("Scheduler received an interrupt signal, stopping services...")
		s.cancel()
	case <-s.ctx.Done():
		// Context 被取消，退出
		log.Info("Scheduler is shutting down...")
	}

	s.Stop()
}

func (s *Scheduler) Send(from, to uint32, content *Content) error {
	val, ok := s.services.Load(to)
	if !ok {
		return fmt.Errorf("service with id %d does not exist", to)
	}

	service := val.(Service)
	msg := &Message{
		From:    from,
		To:      to,
		Content: content,
	}
	if err := service.SendMessage(msg); err != nil {
		return fmt.Errorf("failed to send message to service %d: %v", to, err)
	}
	return nil
}

func (s *Scheduler) Call(from, to uint32, content *Content) (interface{}, error) {
	val, ok := s.services.Load(to)
	if !ok {
		return nil, fmt.Errorf("service with id %d does not exist", to)
	}

	service := val.(Service)
	content.ChanRet = make(chan *RetInfo, 1)
	msg := &Message{
		From:    from,
		To:      to,
		Content: content,
	}
	if err := service.SendMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to send message to service %d: %v", to, err)
	}
	ri := <-content.ChanRet
	return ri.ret, ri.err
}

// PluginService 是一个实现了 Service 接口的插件服务
type PluginService struct {
	*BaseService
}

func NewPluginService(ctx context.Context, scheduler *Scheduler) Service {
	service := NewBaseService(ctx, scheduler)
	return &PluginService{
		BaseService: service.(*BaseService),
	}
}

// Run 重写了 BaseService 的 Run 方法，以提供特定的执行逻辑
func (p *PluginService) Dispatch(wg *sync.WaitGroup) {
	log.Info("PluginService is running", "id", p.GetId())
	p.BaseService.Dispatch(wg) // 调用基类的 Run 方法执行基本的消息处理逻辑
}

// Stop 重写了 BaseService 的 Stop 方法，以提供特定的停止逻辑
func (p *PluginService) Stop() {
	log.Info("PluginService is stopping", "id", p.GetId())
	p.BaseService.Stop() // 调用基类的 Stop 方法来执行取消操作
}
