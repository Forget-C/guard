package guard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"sync"
	"time"
)

var PathError = errors.New("The path cannot be empty ")
var RegisterExistError = errors.New("The current path already exists ")
var NotAllowMultiError = errors.New("The current service does not allow multiple instances ")

// 注册状态记录
type RegisterState struct {
	Option *RegisterOption // 注册时所用的参数
	leaseId clientv3.LeaseID
	ctx context.Context
	client *clientv3.Client
	cancel context.CancelFunc
	CurrentPath string	// 当前注册的路径。当允许多服务时，当前路径是 Option.Path 加上时间戳的。
	handlers
}

// 获取当前的路径
func (r *RegisterState)GetCurrentPath() string {
	return r.CurrentPath
}

// 启动注册
func (r *RegisterState)Start()  {
	r.Keepalive()
}

// 保持注册心跳
func (r *RegisterState)Keepalive()  {
	ch, err := r.keepalive()
	if err != nil {
		r.Option.OnFailedHandler([]byte(r.Option.Path), []byte{}, r, err)
	}

	go func() {
		for {
			select {
			case <- r.ctx.Done():
				for _, handler := range r.onDoneHandler{
					if handler == nil{
						continue
					}
					handler([]byte(r.Option.Path),r)
				}
				return
			case resp, ok := <-ch:
				if !ok {
					for _, handler := range r.onFailedHandler{
						if handler == nil{
							continue
						}
						handler([]byte(r.Option.Path), []byte{}, r, errors.New("Keepalive error "))
					}
				} else {
					for _, handler := range r.onIntervalHandler{
						if handler == nil{
							continue
						}
						handler([]byte(r.Option.Path), r, resp)
					}
				}
			}
		}
	}()
}

// 停止心跳保持
func (r *RegisterState)Stop()  {
	r.cancel()
}

// 注册心跳， 返回租约信息
func (r *RegisterState)keepalive() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	value, err := json.Marshal(r.Option.Info)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Grant(r.ctx, r.Option.Ttl)
	if err != nil {
		return nil, err
	}
	cmp := clientv3.Compare(clientv3.CreateRevision(r.Option.Path), "=", 0)

	if r.Option.Multi {
		r.CurrentPath = fmt.Sprintf("%s/%d", r.Option.Path, time.Now().UnixNano())
		_, err = r.client.Put(r.ctx, r.CurrentPath, string(value), clientv3.WithLease(resp.ID))
		if err != nil {
			return nil, err
		}
	}else {
		r.CurrentPath = r.Option.Path
		put := clientv3.OpPut(r.CurrentPath, string(value), clientv3.WithLease(resp.ID))
		resp, err := r.client.Txn(r.ctx).If(cmp).Then(put).Commit()
		if err != nil {
			return nil, err
		}
		if ! resp.Succeeded {
			return nil, NotAllowMultiError
		}

	}
	r.leaseId = resp.ID

	return r.client.KeepAlive(r.ctx, resp.ID)
}

// 撤销注册
func (r *RegisterState) revoke() error {
	_, err := r.client.Revoke(context.TODO(), r.leaseId)
	return err
}

// 服务注册
type Register struct {
	client *clientv3.Client
	states map[string]*RegisterState
	sync.Mutex
	defaultHandlers handlers
}

// 添加一个服务
func (r *Register)Append(option *RegisterOption) error {
	r.Lock()
	defer r.Unlock()
	if err := option.check(); err != nil {
		return err
	}
	if _, exist := r.states[option.Path]; exist{
		return RegisterExistError
	}
	ctx, cancel := context.WithCancel(context.Background())
	state := RegisterState{Option:option, ctx:ctx, cancel: cancel, client: r.client}
	state.onDoneHandler = append(r.defaultHandlers.onDoneHandler, option.OnDoneHandler)
	state.onFailedHandler = append(r.defaultHandlers.onFailedHandler, option.OnFailedHandler)
	state.onIntervalHandler = append(r.defaultHandlers.onIntervalHandler, option.OnIntervalHandler)
	r.states[option.Path] = &state
	r.states[option.Path].Start()
	return nil
}

// 停止一个服务
func (r *Register)Stop(path string)  {
	r.states[path].Stop()
}

// 停止当前注册的所有服务
func (r *Register)StopAll()  {
	for _, state := range r.states{
		state.Stop()
	}
}

// 注册参数
type RegisterOption struct {
	Path string
	OnFailedHandler FailedHandler
	OnDoneHandler DoneHandler
	OnIntervalHandler IntervalHandler
	Ttl int64
	Info interface{}
	Multi bool
}

func (r *RegisterOption)check() error {
	if len(r.Path) == 0 {
		return PathError
	}
	return nil
}

func NewDefaultRegister(client *clientv3.Client) *Register {
	r := Register{client:client, states:make(map[string]*RegisterState)}
	r.defaultHandlers.AddOnDoneHandler(DefaultOnDoneHandler)
	r.defaultHandlers.AddOnFailedHandler(DefaultOnFailedHandler)
	r.defaultHandlers.AddOnIntervalHandler(DefaultOnIntervalHandler)
	return &r
}

func NewRegister(client *clientv3.Client) *Register {
	r := Register{client:client, states:make(map[string]*RegisterState)}
	return &r
}


