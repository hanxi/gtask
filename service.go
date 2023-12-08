package gtask

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanxi/gtask/config"
	"github.com/hanxi/gtask/log"
)

// rpc 消息
type Message struct {
	From    uint64
	To      uint64
	Content *Content
}

// rpc 消息内容
type Content struct {
	Name    string      // 函数名
	Arg     interface{} // 参数
	Session uint64

	chanRet chan *RetInfo // 返回值
	cb      CbFunc        // AsyncCall 回调函数
	proto   MessageType   // 消息类型
}

// rpc 返回值
type RetInfo struct {
	ret interface{}
	err error
}

// rpc 处理函数
type HandlerFunc func(arg interface{}) interface{}

// AsyncCall 回调函数
type CbFunc func(ret interface{}, err error)

// 定义 Service 接口
type Service interface {
	stop()
	GetID() uint64
	Send(to uint64, content *Content) error                 // 发送消息
	Call(to uint64, content *Content) (interface{}, error)  // 同步rpc
	AsyncCall(to uint64, content *Content, cb CbFunc) error // 异步rpc
	Register(name string, fn HandlerFunc)                   // 注册rpc处理函数

	run(wg *sync.WaitGroup) // 消息处理
	rawSend(msg *Message) error
	getStatus() ServiceStatus
	setStatus(status ServiceStatus)
	setMessageOut(messageOut chan Message)
	getMessageIn() chan Message
}

type ServiceStatus int

const (
	SERVICE_STATUS_CREATE ServiceStatus = iota
	SERVICE_STATUS_INIT
	SERVICE_STATUS_RUNNING
	SERVICE_STATUS_DIE
)

const (
	SERVICE_ID_SCHEDULER uint64 = 1 // 调度服务
)

type MessageType int

const (
	MESSAGE_REQUEST MessageType = iota
	MESSAGE_RESPONSE
)

type BaseService struct {
	id         uint64
	messageIn  chan Message
	messageOut chan Message
	ctx        context.Context
	cancel     context.CancelFunc
	status     ServiceStatus
	handlers   map[string]HandlerFunc
}

var nextSessionId uint64
var nextServiceId uint64 = 1024 // 1~1024 作为特殊服务

func NewSessionId() uint64 {
	return atomic.AddUint64(&nextSessionId, 1)
}
func NewServiceId() uint64 {
	return atomic.AddUint64(&nextServiceId, 1)
}

func NewBaseServiceNoId(ctx context.Context) *BaseService {
	ctx, cancel := context.WithCancel(ctx)
	return &BaseService{
		id:        uint64(0),
		messageIn: make(chan Message, config.C.MsgQueueLen),
		ctx:       ctx,
		cancel:    cancel,
		status:    SERVICE_STATUS_CREATE,
		handlers:  make(map[string]HandlerFunc),
	}
}

func NewBaseService(ctx context.Context) *BaseService {
	s := NewBaseServiceNoId(ctx)
	s.id = NewServiceId()
	return s
}

func (s *BaseService) Register(name string, fn HandlerFunc) {
	if _, ok := s.handlers[name]; ok {
		log.Error("already registered", "name", name)
		return
	}
	s.handlers[name] = fn
}

func (s *BaseService) stop() {
	s.setStatus(SERVICE_STATUS_DIE)
	s.cancel()
}

func (s *BaseService) getMessageIn() chan Message {
	return s.messageIn
}

func (s *BaseService) setMessageOut(messageOut chan Message) {
	s.messageOut = messageOut
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
			log.Error("Service cb not exist", "id", s.GetID())
			return
		}
		ri, ok := content.Arg.(*RetInfo)
		if !ok {
			log.Error("Not RetInfo", "id", s.GetID(), "arg", content.Arg)
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
	log.Info("Service handler ok", "id", s.GetID(), "handler", content.Name, "ret", ret, "arg", content.Arg)
	s.ret(msg, &RetInfo{ret: ret})
}

func (s *BaseService) run(wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("in run ", "service", s)
	s.setStatus(SERVICE_STATUS_RUNNING)
	for {
		select {
		case msg := <-s.messageIn:
			s.exec(&msg)
		case <-s.ctx.Done():
			log.Info("Service is closing", "id", s.GetID())
			return
		}
	}
}

func (s *BaseService) GetID() uint64 {
	return s.id
}

func (s *BaseService) rawSend(msg *Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	select {
	case s.messageOut <- *msg:
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
	content.Session = NewSessionId()
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: content,
	}
	return s.rawSend(msg)
}

func (s *BaseService) response(to uint64, content *Content) error {
	content.proto = MESSAGE_RESPONSE
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: content,
	}
	return s.rawSend(msg)
}

func (s *BaseService) Call(to uint64, content *Content) (interface{}, error) {
	content.chanRet = make(chan *RetInfo, 1)
	content.Session = NewSessionId()
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: content,
	}
	err := s.rawSend(msg)
	if err != nil {
		close(content.chanRet) // 发送失败时关闭 channel
		return nil, err
	}

	timeout := time.Duration(config.C.CallTimeout) * time.Second
	// TODO: 是否需要使用 context.WithTimeout
	select {
	case ri := <-content.chanRet:
		return ri.ret, ri.err
	case <-time.After(timeout):
		close(content.chanRet) // 超时后关闭 channel
		return nil, fmt.Errorf("service %d call to service %d timed out", s.GetID(), to)
	}
}

func (s *BaseService) AsyncCall(to uint64, content *Content, cb CbFunc) error {
	content.Session = NewSessionId()
	content.cb = cb
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: content,
	}
	return s.rawSend(msg)
}

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
