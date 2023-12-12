package gtask

import (
	"context"
	"github.com/hanxi/gtask/log"
	"sync"
	"testing"
	"time"
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
	go bs.run(bs, &wg)
}

func TestSchedulerService(t *testing.T) {
	log.Info("in TestSchedulerService")
	scheduler := newSchedulerService(context.Background())
	scheduler.registerService(scheduler) // 只能在 scheduler 里调用

	s1 := NewPluginService(context.Background())
	s1.Register("f1", func(arg interface{}) interface{} {
		log.Info("in s1 f1", "arg", arg)
		return "return s1 f1"
	})
	ret, err := scheduler.Call(SERVICE_ID_SCHEDULER, Content{Name: "rpcRegisterService", Arg: s1})
	log.Info("after RegisterService", "err", err, "ret", ret)

	scheduler.Send(s1.GetID(), Content{Name: "f1", Arg: "f1arg"})
	s1.Send(scheduler.GetID(), Content{Name: "rpcRegisterService", Arg: s1})

	go func() {
		time.Sleep(2 * time.Second)
		scheduler.stop()
	}()

	scheduler.wait()
}

func BenchmarkNewSessionIdParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			NewSessionId()
		}
	})
}
