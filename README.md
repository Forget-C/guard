[English](README.md)|[中文](README_CN.md)

Guard is an ETCD V3 based service registration and discovery component that was stripped from the production project and is currently in iteration

Support service multi-copy registration and discovery, with rotation training, support load balancing

Support for custom trigger events, you can define multiple and chained execution

## Install

```shell script
go get github.com/Forget-C/guard
```

## Usage

#### Register
##### Multiple copy registration is allowed
```go
path := "service/test_service"
client,_ := clientv3.New(clientv3.Config{
	Endpoints:   []string{"127.0.0.1:2379"},
	DialTimeout: 2 * time.Second,
})
	r := NewDefaultRegister(client)
	err := r.Append(&RegisterOption{Path: path, Info: DataContainer{HostName: "aaaa"}, Multi: true})
```

##### Single-instance service registration
```go
path := "service/test_service"
client,_ = clientv3.New(clientv3.Config{
	Endpoints:   []string{"127.0.0.1:2379"},
	DialTimeout: 2 * time.Second,
})
	r := NewDefaultRegister(client)
	err := r.Append(&RegisterOption{Path: path, Info: DataContainer{HostName: "aaaa"}, Multi: false})
```

##### Cancel a heartbeat registration
```go
r.Stop(path)
```

##### Cancel all heartbeat registration
```go
r.StopAll()
```

#### Discover
##### Add a listener path, and the listener serves multiple instances
```go
path := ""service/test_service"
d := NewDefaultDiscover(client)
err := d.Append(&DiscoverOption{Path:path, Prefix:true})
```
##### Add a listener path and the listener is a single-instance service
```go
path := ""service/test_service"
d := NewDefaultDiscover(client)
err := d.Append(&DiscoverOption{Path:path, Prefix:false})
```
##### Stop a listener
```go
d.Stop(path)
```
##### Stop all listening
```go
d.StopAll()
```
##### Gets the status of a listener
```go
d.PrefixGet(path)
```
Information returned
```go
type DiscoverState struct {
	Option *DiscoverOption // listener parameters 
	CurrentPath string	// The current listening path。
	IsChild bool        // Is a child state. When the listener is multiple copies, the structures of stateful information are all substates
	Data *DataContainer // Contains registration data. When the listener is multiple copies, the value is empty
}
```

##### Gets all the child states of a listening state
This function needs to be used to retrieve data when the listener is multiple copies
```go
d.PrefixGetChildren(path)
```

#### Polling gets the substates
```go
d.PrefixRoundRobinChildren(path)
```