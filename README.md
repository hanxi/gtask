# gtask
模仿ltask的一个玩具

> 仅供学习使用，请勿用于正式环境

## 关键特性
- 一个服务同一时刻只会有一个 goroutine 在运行
- 服务间通信就三种方式 Send Call AsyncCall
- 服务支持从插件里加载

## TODO
- [x] 插件服务加载示例
- [x] Hello World 服务示例
- [x] 实现 bootstrap 服务
- [ ] 实现 console 服务
- [ ] 接入 NATS 实现远程服务 RPC
  - 参考 skynet 一个节点只实现一个 client 和一个 server
  - 协议采用 pb3


## 参考
- https://github.com/cloudwu/ltask
- https://github.com/liangdas/mqant
- https://github.com/name5566/leaf
- https://github.com/bobohume/gonet
- https://github.com/xingshuo/skyline
