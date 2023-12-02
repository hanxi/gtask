package main

import (
	"fmt"
	"sync"
)

// Task 接口定义了可以执行的任务
type Task interface {
	Execute() error
}

// MyTask 是一个实现了 Task 接口的具体任务
type MyTask struct {
	ID int
}

// Execute 是 MyTask 实现 Task 接口的方法
func (t *MyTask) Execute() error {
	fmt.Printf("Executing task #%d\n", t.ID)
	return nil
}

// Message 定义了服务之间交换的消息结构
type Message struct {
	From    string
	To      string
	Content interface{}
}

// Service 定义了一个可以运行任务和处理消息的服务
type Service struct {
	ID         string
	TaskQueue  chan Task
	MessageIn  chan Message
	MessageOut chan Message
}

// NewService 创建并初始化一个新的 Service
func NewService(id string, messageOut chan Message) *Service {
	return &Service{
		ID:         id,
		TaskQueue:  make(chan Task),
		MessageIn:  make(chan Message),
		MessageOut: messageOut,
	}
}

// Run 启动服务的主循环
func (s *Service) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case task := <-s.TaskQueue:
			if err := task.Execute(); err != nil {
				fmt.Printf("Error executing task: %v\n", err)
			}
		case msg := <-s.MessageIn:
			fmt.Printf("Service %s received a message from %s: %+v\n", s.ID, msg.From, msg.Content)
			// 根据消息内容处理消息...
		}
	}
}

// Scheduler 调度器用于任务调度和服务消息路由
type Scheduler struct {
	services   map[string]*Service
	messageOut chan Message
	wg         sync.WaitGroup
}

// NewScheduler 创建一个新的 Scheduler
func NewScheduler() *Scheduler {
	return &Scheduler{
		services:   make(map[string]*Service),
		messageOut: make(chan Message),
	}
}

// RegisterService 注册一个新的 Service 到调度器
func (s *Scheduler) RegisterService(service *Service) {
	s.services[service.ID] = service
	s.wg.Add(1)
	go service.Run(&s.wg)
}

// Start 开始调度器的消息路由功能
func (s *Scheduler) Start() {
	go func() {
		for msg := range s.messageOut {
			if service, ok := s.services[msg.To]; ok {
				service.MessageIn <- msg
			} else {
				fmt.Printf("No service found with ID %s\n", msg.To)
			}
		}
	}()
}

// Stop 停止调度器，并等待所有服务完成
func (s *Scheduler) Stop() {
	close(s.messageOut) // 关闭消息输出通道，这会导致路由循环结束
	s.wg.Wait()         // 等待所有服务完成它们的工作
}

// Send 发送消息到指定的服务
func (s *Service) Send(to string, content interface{}) {
	s.MessageOut <- Message{From: s.ID, To: to, Content: content}
}

func main() {
	scheduler := NewScheduler()

	// 创建并注册服务
	serviceA := NewService("A", scheduler.messageOut)
	serviceB := NewService("B", scheduler.messageOut)
	scheduler.RegisterService(serviceA)
	scheduler.RegisterService(serviceB)

	// 开始调度器
	scheduler.Start()

	// 发送任务到服务A
	serviceA.TaskQueue <- &MyTask{ID: 1}

	// 服务A向服务B发送消息
	serviceA.Send("B", "Hi from A")

	// 停止调度器
	scheduler.Stop()
}
