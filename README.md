# gtask
模仿ltask的一个玩具

## 关键特性
- 一个服务同一时刻只会有一个 goroutine 在运行
- 服务间通信就三种方式 Send Call AsyncCall
- 服务支持从插件里加载

## TODO
- 插件服务加载示例
- Hello World 服务示例
- 接入 NATS 实现远程服务 RPC
  - 参考 skynet 一个节点只实现一个 client 和一个 server
  - 协议采用 pb3

