package gtask

import (
	"context"
	"github.com/hanxi/gtask/log"
	"sync"
	"testing"
)

func TestBasicService(t *testing.T) {
	log.Info("in TestBasicService")
	bs := NewBaseService(context.Background())
	bs.Register("bs1", func(arg interface{}) interface{} {
		log.Info("in bs1", "arg", arg)
		return "return bs1"
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go bs.run(&wg)
}

func TestSchedulerService(t *testing.T) {
	log.Info("in TestSchedulerService")
	scheduler := NewSchedulerService(context.Background())
	scheduler.registerService(scheduler) // 只能在 scheduler 里调用

	s1 := NewPluginService(context.Background())
	s1.Register("f1", func(arg interface{}) interface{} {
		log.Info("in s1 f1", "arg", arg)
		return "return s1 f1"
	})
	scheduler.RegisterService(s1) // 任意 goroutine 里调用都可以
	scheduler.Send(s1.GetID(), &Content{Name: "f1", Arg: "f1arg"})
	s1.Send(scheduler.GetID(), &Content{Name: "registerService", Arg: &registerServiceArg{s: scheduler, service: s1}})

	// 开服务 spawn
	// 关服务 kill
	// 命名 register
	// 查询 queryservice
	// 创建唯一服务 uniqueservice
	// 发送消息

	scheduler.wait()
}

func BenchmarkNewSessionIdParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			NewSessionId()
		}
	})
}

/*
func TestService(t *testing.T) {
	log.Info("debug1")
	scheduler := NewScheduler()
	s1 := NewBaseService(context.Background(), scheduler)
	s2 := NewPluginService(context.Background(), scheduler)
	id1, _ := scheduler.RegisterService(s1)
	err := scheduler.Dispatch(s1)
	if err != nil {
		log.Error("Dispatch s1 failed.", "err", err)
	}
	id2, _ := scheduler.RegisterService(s2)
	err = scheduler.Dispatch(s2)
	if err != nil {
		log.Error("Dispatch s2 failed.", "err", err)
	}

	s1.Register("s1func", func(arg interface{}) interface{} {
		log.Info("s1 in handler", "arg", arg)

		ret, err := s1.Call(id2, &Content{Name: "s2func", Arg: "call s2 arg"})
		if err != nil {
			log.Error("s1 call s2 failed.", "err", err)
		}
		log.Info("s1 call s2 return", "ret", ret)

		return "s1func return ret"
	})
	s2.Register("s2func", func(arg interface{}) interface{} {
		log.Info("s2 in handler", "arg", arg)
		time.Sleep(2 * time.Second)
		return "s2func return ret"
	})

	log.Info("debug2")

	err = scheduler.Send(id1, id2, &Content{Name: "s2func", Arg: "send s1 arg"})
	if err != nil {
		log.Error("s1 send s2 failed.", "err", err)
	}
	err = s2.Send(id1, &Content{Name: "s1func", Arg: "send s2 arg"})
	if err != nil {
		log.Error("s2 send s1 failed.", "err", err)
	}
	log.Info("debug3")

	err = s2.AsyncCall(id1, &Content{Name: "s1func", Arg: "AsyncCall s1 arg"}, func(ret interface{}, err error) {
		log.Info("s2 AsyncCall s1 back", "ret", ret, "err", err)
	})
	if err != nil {
		log.Error("s2 AsyncCall s1 failed.", "err", err)
	}
	log.Info("debug4")

	scheduler.Loop()
	fmt.Println("debug end")
}
*/
