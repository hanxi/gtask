package gtask

import (
	"context"
	"fmt"
	"testing"
)

func TestService(t *testing.T) {
	queueSize := uint32(100)

	fmt.Println("debug1")
	scheduler := NewScheduler()
	s1 := NewBaseService(context.Background(), scheduler, queueSize)
	s2 := NewPluginService(context.Background(), scheduler, queueSize)
	s2.Handler("hello", func(args interface{}) interface{} {
		fmt.Println("in hello handler")
		return "hello world"
	})
	s1.Handler("world", func(args interface{}) interface{} {
		fmt.Println("in world handler")
		return "hello world"
	})
	id1, _ := scheduler.RegisterService(s1)
	id2, _ := scheduler.RegisterService(s2)
	fmt.Println("debug2")

	err := scheduler.Send(id1, id2, Content{Name: "hello", Args: "helloarg"})
	if err != nil {
		fmt.Println(err)
	}
	err = s2.Send(id1, Content{Name: "world", Args: "worldarg"})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("debug3")

	scheduler.Loop()
	fmt.Println("debug4")
}
