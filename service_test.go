package gotask

import (
	"context"
	"fmt"
	"testing"
)

func TestService(t *testing.T) {
	queueSize := uint32(100)

	fmt.Println("debug1")
	scheduler := NewScheduler()
	s1 := NewBaseService(queueSize, context.Background())
	s2 := NewPluginService(queueSize, context.Background())
	id1 := scheduler.RegisterService(s1)
	id2 := scheduler.RegisterService(s2)
	fmt.Println("debug2")

	err := scheduler.Send(id1, id2, "hello")
	if err != nil {
		fmt.Println(err)
	}
	err = scheduler.Send(id2, id1, "world")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("debug3")

	scheduler.Run()
	fmt.Println("debug4")
}
