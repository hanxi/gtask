package gtask

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask/log"
	"testing"
	"time"
)

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
