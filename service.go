package gotask

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

// 定义 Service 接口
type Service interface {
	Run(wg *sync.WaitGroup)
	Stop()
	GetId() uint32
	SetId(id uint32)
	SendMessage(msg Message) error
}

type BaseService struct {
	id      uint32
	chanMsg chan Message
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewBaseService(msgSize uint32, ctx context.Context) *BaseService {
	ctx, cancel := context.WithCancel(ctx)
	return &BaseService{
		id:      uint32(0),
		chanMsg: make(chan Message, msgSize),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *BaseService) Stop() {
	s.cancel()
}

func (s *BaseService) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case msg := <-s.chanMsg:
			fmt.Printf("Service %d received a message from %d: %+v\n", s.GetId(), msg.From, msg.Content)
		case <-s.ctx.Done():
			fmt.Println("Service is closing")
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

func (s *BaseService) SendMessage(msg Message) error {
	select {
	case s.chanMsg <- msg:
		return nil
	default:
		return fmt.Errorf("message queue is full")
	}
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
	service.(Service).SetId(id) // 设置服务的 ID
	s.services.Store(id, service)
	return id
}

func (s *Scheduler) Stop() {
	s.services.Range(func(key, value interface{}) bool {
		service := value.(Service)
		service.Stop()
		return true
	})
	s.wg.Wait()
	s.cancel() // 发出关闭调度器的信号
}

func (s *Scheduler) Run() {
	s.services.Range(func(key, value interface{}) bool {
		s.wg.Add(1)
		go value.(Service).Run(&s.wg)
		return true
	})

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

func (s *Scheduler) Send(from, to uint32, content interface{}) (err error) {
	val, ok := s.services.Load(to)
	if !ok {
		return fmt.Errorf("service with id %d does not exist", to)
	}

	service := val.(Service)
	msg := Message{From: from, To: to, Content: content}
	if err := service.SendMessage(msg); err != nil {
		return fmt.Errorf("failed to send message to service %d: %v", to, err)
	}
	return nil
}

// PluginService 是一个实现了 Service 接口的插件服务
type PluginService struct {
	*BaseService
}

// NewPluginService 创建一个新的 PluginService 实例
func NewPluginService(msgSize uint32, ctx context.Context) *PluginService {
	baseService := NewBaseService(msgSize, ctx)
	return &PluginService{
		BaseService: baseService,
	}
}

// Run 重写了 BaseService 的 Run 方法，以提供特定的执行逻辑
func (p *PluginService) Run(wg *sync.WaitGroup) {
	fmt.Printf("PluginService %d is running\n", p.GetId())
	p.BaseService.Run(wg) // 调用基类的 Run 方法执行基本的消息处理逻辑
}

// Stop 重写了 BaseService 的 Stop 方法，以提供特定的停止逻辑
func (p *PluginService) Stop() {
	fmt.Printf("PluginService %d is stopping\n", p.GetId())
	p.BaseService.Stop() // 调用基类的 Stop 方法来执行取消操作
}
