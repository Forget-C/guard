guard 是一个基于etcd v3的服务注册与发现组件, 从生产项目中剥离出， 并且目前还在迭代中

支持服务多副本注册与发现， 并带有轮训器，支持负载均衡

支持自定义触发事件， 可以定义多个并链式执行  

## 安装

···
go get github.com/Forget-C/guard
···

## 使用

#### 注册
##### 允许多副本注册
```go
path := "service/test_service"
client,_ := clientv3.New(clientv3.Config{
	Endpoints:   []string{"127.0.0.1:2379"},
	DialTimeout: 2 * time.Second,
})
	r := NewDefaultRegister(client)
	err := r.Append(&RegisterOption{Path: path, Info: DataContainer{HostName: "aaaa"}, Multi: true})
```

##### 单实例服务注册
```go
path := "service/test_service"
client,_ = clientv3.New(clientv3.Config{
	Endpoints:   []string{"127.0.0.1:2379"},
	DialTimeout: 2 * time.Second,
})
	r := NewDefaultRegister(client)
	err := r.Append(&RegisterOption{Path: path, Info: DataContainer{HostName: "aaaa"}, Multi: false})
```

##### 取消某个心跳注册
```go
r.Stop(path)
```

##### 取消全部心跳注册
```go
r.StopAll()
```

#### 发现
##### 添加监听路径, 且被监听者为多实例服务
```go
path := ""service/test_service"
d := NewDefaultDiscover(client)
err := d.Append(&DiscoverOption{Path:path, Prefix:true})
```
##### 添加监听路径, 且被监听者为单实例服务
```go
path := ""service/test_service"
d := NewDefaultDiscover(client)
err := d.Append(&DiscoverOption{Path:path, Prefix:false})
```
##### 停止某个监听
```go
d.Stop(path)
```
##### 停止全部监听
```go
d.StopAll()
```
##### 获取某个监听的状态
```go
d.PrefixGet(path)
```
返回的信息
```go
type DiscoverState struct {
	Option *DiscoverOption // 监听参数
	CurrentPath string	// 当前的监听路径。
	IsChild bool        // 是否为子状态。 当被监听者为多副本时， 有状态信息的结构体均为子状态
	Data *DataContainer // 包含的注册数据。 当被监听者为多副本时，值为空
}
```

##### 获取某个监听状态的所有子状态
当被监听者为多副本时, 需要使用此函数获取数据
```go
d.PrefixGetChildren(path)
```

#### 轮询获取子状态
```go
d.PrefixRoundRobinChildren(path)
```