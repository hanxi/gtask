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
	GetID() uint64
	Send(to uint64, content Content) error                 // 发送消息
	Call(to uint64, content Content) (interface{}, error)  // 同步rpc
	AsyncCall(to uint64, content Content, cb CbFunc) error // 异步rpc
	Register(name string, fn HandlerFunc)                  // 注册rpc处理函数
	GetStatus() ServiceStatus
	OnInit()

	run(c Service, wg *sync.WaitGroup) // 消息处理
	stop()
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

func NewBaseServiceWithId(ctx context.Context, id uint64) *BaseService {
	ctx, cancel := context.WithCancel(ctx)
	s := &BaseService{
		id:        id,
		messageIn: make(chan Message, config.C.MsgQueueLen),
		ctx:       ctx,
		cancel:    cancel,
		status:    SERVICE_STATUS_CREATE,
		handlers:  make(map[string]HandlerFunc),
	}
	s.Register("rpcStop", s.rpcStop)
	return s
}

func NewBaseService(ctx context.Context) *BaseService {
	s := NewBaseServiceWithId(ctx, 0)
	s.id = NewServiceId()
	return s
}

func (s *BaseService) Register(name string, fn HandlerFunc) {
	if _, ok := s.handlers[name]; ok {
		log.Debug("already registered will override", "name", name, "old", s.handlers[name])
	}
	s.handlers[name] = fn
	log.Debug("Register", "name", name, "fn", fn)
}

func (s *BaseService) stop() {
	log.Debug("BaseService stop", "id", s.id)
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
		newContent := Content{
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

	log.Debug("exec", "msg", msg)

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

func (s *BaseService) OnInit() {
	log.Debug("OnInit", "id", s.id)
}

func (s *BaseService) run(c Service, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("in run ", "service", s, "c", c)
	s.setStatus(SERVICE_STATUS_RUNNING)
	c.OnInit()
	for {
		select {
		case msg := <-s.messageIn:
			s.exec(&msg)
		case <-s.ctx.Done():
			log.Info("Service is closing", "id", s.GetID())
			return
		}
	}
	log.Info("end run", "id", s.GetID())
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

	log.Debug("rawSend", "msg", msg)
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

func (s *BaseService) GetStatus() ServiceStatus {
	return s.status
}

func (s *BaseService) setStatus(status ServiceStatus) {
	s.status = status
}

func (s *BaseService) Send(to uint64, content Content) error {
	content.Session = NewSessionId()
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: &content,
	}
	return s.rawSend(msg)
}

func (s *BaseService) AsyncCall(to uint64, content Content, cb CbFunc) error {
	content.Session = NewSessionId()
	content.cb = cb
	content.proto = MESSAGE_REQUEST
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: &content,
	}
	return s.rawSend(msg)
}

func (s *BaseService) response(to uint64, content Content) error {
	content.proto = MESSAGE_RESPONSE
	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: &content,
	}
	return s.rawSend(msg)
}

func (s *BaseService) Call(to uint64, content Content) (interface{}, error) {
	content.chanRet = make(chan *RetInfo, 1)
	content.Session = NewSessionId()
	content.proto = MESSAGE_REQUEST

	msg := &Message{
		From:    s.GetID(),
		To:      to,
		Content: &content,
	}

	err := s.rawSend(msg)
	if err != nil {
		// 安全地关闭 channel
		s.closeRetChannelSafely(content.chanRet)
		return nil, err
	}

	// 使用 BaseService 内部的 ctx 创建一个带有超时的 context
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(config.C.CallTimeout)*time.Second)
	defer cancel() // 确保在操作完成或超时后释放资源

	select {
	case ri := <-content.chanRet:
		return ri.ret, ri.err
	case <-ctx.Done():
		// 超时处理，安全地关闭 channel
		s.closeRetChannelSafely(content.chanRet)
		// 检查超时或 BaseService 停止的原因
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("service %d call to service %d timed out", s.GetID(), to)
		}
		return nil, fmt.Errorf("service %d stopped while waiting for response", s.GetID())
	}
}

func (s *BaseService) closeRetChannelSafely(ch chan *RetInfo) {
	select {
	case _, ok := <-ch:
		if !ok {
			// Channel 已关闭，不要再次关闭它
			return
		}
	default:
	}
	close(ch)
}

type rpcStopArg struct {
	s Service
}

func (s *BaseService) rpcStop(arg interface{}) interface{} {
	s.stop()
	return nil
}
