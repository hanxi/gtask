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
	"time"
)

type Message struct {
	From    uint64
	To      uint64
	Content *Content
}

type Content struct {
	Name string
	Arg  interface{}

	chanRet chan *RetInfo
	Session uint64
	cb      CbFunc
	proto   MessageType
}

type RetInfo struct {
	ret interface{}
	err error
}

type HandlerFunc func(arg interface{}) interface{}
type CbFunc func(ret interface{}, err error)

// 定义 Service 接口
type Service interface {
	Stop()
	GetId() uint64
	Send(to uint64, content *Content) error                 // 发送消息
	Call(to uint64, content *Content) (interface{}, error)  // 同步rpc
	AsyncCall(to uint64, content *Content, cb CbFunc) error // 异步rpc
	Register(name string, fn HandlerFunc)

	sendMessage(msg *Message) error
	dispatch(wg *sync.WaitGroup)
	setId(id uint64)
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

type MessageType int

const (
	MESSAGE_REQUEST MessageType = iota
	MESSAGE_RESPONSE
)

type BaseService struct {
	id        uint64
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
		id:        uint64(0),
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
	s.setStatus(SERVICE_STATUS_DIE)
	s.cancel()
}

func (s *BaseService) ret(msg *Message, ri *RetInfo) (err error) {
	content := msg.Content
	if content.chanRet == nil && content.cb == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	if content.cb == nil {
		content.chanRet <- ri
	} else {
		newContent := &Content{
			Arg:     ri,
			Session: content.Session,
			cb:      content.cb,
		}
		err = s.response(msg.From, newContent)
	}
	return
}

func (s *BaseService) exec(msg *Message) {
	defer func() {
		if r := recover(); r != nil {
			if config.C.StackBufLen > 0 {
				buf := make([]byte, config.C.StackBufLen)
				l := runtime.Stack(buf, false)
				log.Error("exec failed", "r", r, "buf", buf[:l])
			} else {
				log.Error("exec failed", "r", r)
			}

			s.ret(msg, &RetInfo{err: fmt.Errorf("%v", r)})
		}
	}()

	// 处理 AsyncCall
	content := msg.Content
	if content.proto == MESSAGE_RESPONSE {
		cbFunc := content.cb
		if cbFunc == nil {
			log.Error("Service cb not exist", "id", s.GetId())
			return
		}
		ri, ok := content.Arg.(*RetInfo)
		if !ok {
			log.Error("Not RetInfo", "id", s.GetId(), "arg", content.Arg)
			return
		}
		cbFunc(ri.ret, ri.err)
		return
	}

	// execute
	handFunc, exist := s.handlers[content.Name]
	if !exist {
		err := fmt.Errorf("unknow handler:%s", content.Name)
		log.Error("unknow handler", "name", content.Name)
		s.ret(msg, &RetInfo{err: err})
		return
	}

	ret := handFunc(content.Arg)
	log.Info("Service handler ok", "id", s.GetId(), "handler", content.Name, "ret", ret, "arg", content.Arg)
	s.ret(msg, &RetInfo{ret: ret})
}

func (s *BaseService) dispatch(wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("in dispatch", "service", s)
	s.setStatus(SERVICE_STATUS_RUNNING)
	for {
		select {
		case msg := <-s.chanMsg:
			s.exec(&msg)
		case <-s.ctx.Done():
			log.Info("Service is closing")
			return
		}
	}
}

func (s *BaseService) GetId() uint64 {
	return s.id
}

func (s *BaseService) setId(id uint64) {
	s.id = id
}

func (s *BaseService) sendMessage(msg *Message) (err error) {
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

func (s *BaseService) getStatus() ServiceStatus {
	return s.status
}

func (s *BaseService) setStatus(status ServiceStatus) {
	s.status = status
}

func (s *BaseService) Send(to uint64, content *Content) error {
	return s.scheduler.Send(s.id, to, content)
}

func (s *BaseService) response(to uint64, content *Content) error {
	return s.scheduler.response(s.id, to, content)
}

func (s *BaseService) Call(to uint64, content *Content) (interface{}, error) {
	return s.scheduler.Call(s.id, to, content)
}

func (s *BaseService) AsyncCall(to uint64, content *Content, cb CbFunc) error {
	return s.scheduler.AsyncCall(s.id, to, content, cb)
}

type Scheduler struct {
	nextServiceId uint64
	nextSessionId uint64
	services      sync.Map
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) NewSessionId() uint64 {
	return atomic.AddUint64(&s.nextSessionId, 1)
}

func (s *Scheduler) RegisterService(service Service) (uint64, error) {
	id := atomic.AddUint64(&s.nextServiceId, 1)
	_, exist := s.services.Load(id)
	if exist {
		return 0, fmt.Errorf("Service with ID %d already exists", id)
	}
	service.setId(id) // 设置服务的 ID
	s.services.Store(id, service)
	service.setStatus(SERVICE_STATUS_INIT)
	return id, nil
}

func (s *Scheduler) Stop() {
	s.services.Range(func(key, value interface{}) bool {
		id := key.(uint64)
		service := value.(Service)
		service.Stop()
		s.services.Delete(id)
		return true
	})

	s.wg.Wait()
	s.cancel() // 发出关闭调度器的信号
}

func (s *Scheduler) Dispatch(service Service) error {
	log.Info("Dispatch", "service", service)
	if service.getStatus() != SERVICE_STATUS_INIT {
		return fmt.Errorf("Scheduler RegisterService failed. id:%d", service.GetId())
	}
	s.wg.Add(1)
	go service.dispatch(&s.wg)
	return nil
}

func (s *Scheduler) Loop() {
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

func (s *Scheduler) rawSend(msg *Message) error {
	val, ok := s.services.Load(msg.To)
	if !ok {
		return fmt.Errorf("service with id %d does not exist", msg.To)
	}
	service := val.(Service)
	if err := service.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send message to service %d: %v", msg.To, err)
	}
	return nil
}

func (s *Scheduler) Send(from, to uint64, content *Content) error {
	content.Session = s.NewSessionId()
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    from,
		To:      to,
		Content: content,
	}
	return s.rawSend(msg)
}

func (s *Scheduler) response(from, to uint64, content *Content) error {
	content.Session = s.NewSessionId()
	content.proto = MESSAGE_RESPONSE
	msg := &Message{
		From:    from,
		To:      to,
		Content: content,
	}
	return s.rawSend(msg)
}

func (s *Scheduler) Call(from, to uint64, content *Content) (interface{}, error) {
	content.chanRet = make(chan *RetInfo, 1)
	content.Session = s.NewSessionId()
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    from,
		To:      to,
		Content: content,
	}
	err := s.rawSend(msg)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(config.C.CallTimeout) * time.Second
	select {
	case ri := <-content.chanRet:
		return ri.ret, ri.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("call to service %d timed out", to)
	}
}

func (s *Scheduler) AsyncCall(from, to uint64, content *Content, cb CbFunc) error {
	content.Session = s.NewSessionId()
	content.cb = cb
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    from,
		To:      to,
		Content: content,
	}
	return s.rawSend(msg)
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

// Stop 重写了 BaseService 的 Stop 方法，以提供特定的停止逻辑
func (p *PluginService) Stop() {
	log.Info("PluginService is stopping", "id", p.GetId())
	p.BaseService.Stop() // 调用基类的 Stop 方法来执行取消操作
}
