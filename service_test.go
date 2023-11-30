package gtask

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask/log"
	"testing"
)

func TestService(t *testing.T) {
	log.Info("debug1")
	scheduler := NewScheduler()
	s1 := NewBaseService(context.Background(), scheduler)
	s2 := NewPluginService(context.Background(), scheduler)
	id1, _ := scheduler.RegisterService(s1)
	id2, _ := scheduler.RegisterService(s2)

	s1.Register("world", func(arg interface{}) interface{} {
		log.Info("in world handler", "arg", arg)

		ret, err := s1.Call(id2, &Content{Name: "hello", Args: "hellocallarg"})
		if err != nil {
			log.Error("call failed.", "err", err)
		}
		log.Info("debug4", "ret", ret)

		return "hello world"
	})
	s2.Register("hello", func(arg interface{}) interface{} {
		log.Info("in hello handler", "arg", arg)
		return "hello world"
	})

	log.Info("debug2")

	err := scheduler.Send(id1, id2, &Content{Name: "hello", Args: "helloarg"})
	if err != nil {
		log.Error("send failed.", "err", err)
	}
	err = s2.Send(id1, &Content{Name: "world", Args: "worldarg"})
	if err != nil {
		log.Error("send failed.", "err", err)
	}
	log.Info("debug3")

	scheduler.Loop()
	fmt.Println("debug end")
}
