# gtask
模仿ltask的一个玩具

## 关键特性
- 一个服务同一时刻只会有一个 goroutine 在运行
- 服务间通信就三种方式 Send Call AsyncCall
- 服务支持从插件里加载
