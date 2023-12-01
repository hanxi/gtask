package main

import (
	"context"
	"github.com/hanxi/gtask"
	"github.com/hanxi/gtask/log"
	"plugin"
)

func main() {
	scheduler := gtask.NewScheduler()

	s1 := gtask.NewBaseService(context.Background(), scheduler)
	s2 := gtask.NewPluginService(context.Background(), scheduler)
	id1, _ := scheduler.RegisterService(s1)
	id2, _ := scheduler.RegisterService(s2)

	s1.Register("world", func(arg interface{}) interface{} {
		log.Info("in world handler")
		return "hello world"
	})
	s2.Register("hello", func(arg interface{}) interface{} {
		log.Info("in hello handler")
		return "hello world"
	})
	log.Info("debug2")

	err := scheduler.Send(id1, id2, &gtask.Content{Name: "hello", Arg: "helloarg"})
	if err != nil {
		log.Error("send filed.", "err", err)
	}
	err = s2.Send(id1, &gtask.Content{Name: "world", Arg: "worldarg"})
	if err != nil {
		log.Error("send filed.", "err", err)
	}
	log.Info("debug3")

	p, err := plugin.Open("myplugin.so") // 打开插件文件
	if err != nil {
		panic(err)
	}

	newFunc, err := p.Lookup("New") // 查找插件导出的New函数
	if err != nil {
		panic(err)
	}

	newServiceFunc, ok := newFunc.(func(ctx context.Context, scheduler *gtask.Scheduler) gtask.Service) // 类型断言为正确的函数签名
	if !ok {
		panic("Plugin has no 'New' function of the correct type")
	}

	s3 := newServiceFunc(context.Background(), scheduler) // 调用函数获取Service实例
	id3, _ := scheduler.RegisterService(s3)
	log.Info("debug4")

	err = s3.Send(id1, &gtask.Content{Name: "world", Arg: "worldarg"})
	if err != nil {
		log.Error("send filed.", "err", err)
	}
	log.Info("debug5")

	err = s1.Send(id3, &gtask.Content{Name: "world", Arg: "worldarg"})
	if err != nil {
		log.Error("send filed.", "err", err)
	}
	log.Info("debug6")

	scheduler.Loop()
}
