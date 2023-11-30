package main

import (
	"context"
	"fmt"
	"github.com/hanxi/gtask"
	"plugin"
)

func main() {
	queueSize := uint32(100)
	scheduler := gtask.NewScheduler()

	s1 := gtask.NewBaseService(context.Background(), scheduler, queueSize)
	s2 := gtask.NewPluginService(context.Background(), scheduler, queueSize)
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

	err := scheduler.Send(id1, id2, gtask.Content{Name: "hello", Args: "helloarg"})
	if err != nil {
		fmt.Println(err)
	}
	err = s2.Send(id1, gtask.Content{Name: "world", Args: "worldarg"})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("debug3")

	p, err := plugin.Open("myplugin.so") // 打开插件文件
	if err != nil {
		panic(err)
	}

	newFunc, err := p.Lookup("New") // 查找插件导出的New函数
	if err != nil {
		panic(err)
	}

	newServiceFunc, ok := newFunc.(func(ctx context.Context, scheduler *gtask.Scheduler, msgSize uint32) gtask.Service) // 类型断言为正确的函数签名
	if !ok {
		panic("Plugin has no 'New' function of the correct type")
	}

	s3 := newServiceFunc(context.Background(), scheduler, queueSize) // 调用函数获取Service实例
	id3, _ := scheduler.RegisterService(s3)
	fmt.Println("debug4")

	err = s3.Send(id1, gtask.Content{Name: "world", Args: "worldarg"})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("debug5")

	err = s1.Send(id3, gtask.Content{Name: "world", Args: "worldarg"})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("debug6")

	scheduler.Loop()
}
