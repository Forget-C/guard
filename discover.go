package guard

import (
	"context"
	"errors"
	"github.com/coreos/etcd/clientv3"
	"sync"
	"sync/atomic"
)

var DiscoverExistError = errors.New("The current path already exists ")

type Discover struct {
	client *clientv3.Client
	discoverStates
	defaultHandlers handlers
	sync.Mutex
}

// 监听的状态集
type discoverStates struct {
	states map[string]*DiscoverState
	lastN uint64
}

// 使用前缀获取状态信息， 当被监听者为单实例时使用此函数
func (d *discoverStates)PrefixGet(prefix string) (*DiscoverState, bool) {
	state, exist := d.states[prefix]
	return state, exist
}
// 使用前缀获取状态信息， 当被监听者为多实例时使用此函数
func (d *discoverStates)PrefixGetChildren(prefix string) *DiscoverStateChildren {
	state, exist := d.PrefixGet(prefix)
	if exist{
		return state.children
	}
	return nil
}

// 使用前缀获取状态信息， 当被监听者为多实例时， 且期望轮询时 使用此函数
func (d *discoverStates)PrefixRoundRobinChildren(prefix string) *DiscoverState  {
	children := d.PrefixGetChildren(prefix)
	if children != nil && children.Count() > 0{
		index := atomic.AddUint64(&d.lastN, 1) % uint64(children.Count())
		atomic.StoreUint64(&d.lastN, index)
		return children.IndexGet(int(index))
	}
	return nil
}

// 子状态集
type DiscoverStateChildren struct {
	states map[string]*DiscoverState	// 路径对应状态
	index []string						// 路径列表
	sync.RWMutex
}

// 当前子状态集的长度计数
func (c DiscoverStateChildren) Count() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.index)
}

// 添加子状态
func (c *DiscoverStateChildren) add(path string, state *DiscoverState) {
	c.Lock()
	defer c.Unlock()
	c.states[path] = state
	c.index = append(c.index, path)
}

// 删除子状态
func (c *DiscoverStateChildren) del(path string) {
	c.Lock()
	defer c.Unlock()
	delete(c.states, path)
	for x, y := range c.index {
		if y == path {
			c.index = append(c.index[:x], c.index[x+1:]...)
			return
		}
	}
}

// 获取子状态
func (c DiscoverStateChildren) Get(path string) *DiscoverState {
	c.RLock()
	defer c.RUnlock()
	return c.states[path]
}

// 获取所有子状态
func (c DiscoverStateChildren) All() map[string]*DiscoverState {
	c.RLock()
	defer c.RUnlock()
	return c.states
}

// 通过索引获取子状态
func (c DiscoverStateChildren) IndexGet(i int) *DiscoverState {
	c.RLock()
	defer c.RUnlock()
	n := c.index[i]
	return c.states[n]
}

// 监听状态
type DiscoverState struct {
	Option *DiscoverOption 	// 监听参数
	ctx context.Context
	client *clientv3.Client
	cancel context.CancelFunc
	children *DiscoverStateChildren
	CurrentPath string		// 当前的监听路径。
	IsChild bool			// 是否为子状态
	Data *DataContainer		// 数据
	handlers
}

// 获取当前的路径
func (d *DiscoverState)GetCurrentPath() string {
	return d.CurrentPath
}

// 启动监听
func (d *DiscoverState)Start()  {
	var value *clientv3.GetResponse
	var err error
	if d.Option.Prefix{
		value, err = d.client.Get(context.TODO(), d.Option.Path, clientv3.WithPrefix())
	}else{
		value, err = d.client.Get(context.TODO(), d.Option.Path)
	}
	if err == nil && len(value.Kvs) >0 {
		for _, kv := range value.Kvs {
			for _, handler := range d.onPutHandlers{
				if handler == nil{
					continue
				}
				handler(kv.Key, kv.Value, d)
			}
		}
	}
	go func() {
		for true {
			select {
			case <- d.ctx.Done():
				for _, handler := range d.onDoneHandler{
					if handler == nil{
						continue
					}
					handler([]byte(d.Option.Path),d)
				}
				return
			default:
				var rch clientv3.WatchChan
				if d.Option.Prefix{
					rch = d.client.Watch(d.ctx, d.Option.Path, clientv3.WithPrefix()) //阻塞在这里，如果没有key里没有变化，就一直停留在这里
				}else{
					rch = d.client.Watch(d.ctx, d.Option.Path)
				}

				for wResp := range rch {
					for _, ev := range wResp.Events {
						switch ev.Type {
						case clientv3.EventTypePut:
							for _, handler := range d.onPutHandlers{
								if handler == nil{
									continue
								}
								handler(ev.Kv.Key, ev.Kv.Value, d)
							}
						case clientv3.EventTypeDelete:
							for _, handler := range d.onDelHandlers{
								if handler == nil{
									continue
								}
								handler(ev.Kv.Key, ev.Kv.Value, d)
							}
						}
					}
				}
			}
		}
	}()
}

// 停止监听
func (d *DiscoverState)Stop()  {
	d.cancel()
}

// 追加监听
func (d *Discover)Append(option *DiscoverOption) error {
	d.Lock()
	defer d.Unlock()
	if err := option.check(); err != nil {
		return err
	}
	if _, exist := d.PrefixGet(option.Path); exist{
		return DiscoverExistError
	}
	ctx, cancel := context.WithCancel(context.Background())
	state := DiscoverState{Option:option, ctx:ctx, cancel: cancel, client: d.client, children:&DiscoverStateChildren{states:make(map[string]*DiscoverState)}, IsChild:false}
	state.onDelHandlers = append(d.defaultHandlers.onDelHandlers, option.OnDelHandler)
	state.onPutHandlers = append(d.defaultHandlers.onPutHandlers, option.OnPutHandler)
	state.onFailedHandler = append(d.defaultHandlers.onFailedHandler, option.OnFailedHandler)
	state.onDoneHandler = append(d.defaultHandlers.onDoneHandler, option.OnDoneHandler)
	d.states[option.Path] = &state
	d.states[option.Path].Start()
	return nil
}

// 添加监听的参数选项
type DiscoverOption struct {
	Path string
	Prefix bool
	OnPutHandler ChangedHandler
	OnDelHandler ChangedHandler
	OnFailedHandler FailedHandler
	OnDoneHandler DoneHandler
}

// 检查参数合法
func (r *DiscoverOption)check() error {
	if len(r.Path) == 0 {
		return PathError
	}
	return nil
}

func NewDefaultDiscover(client *clientv3.Client) *Discover {
	d := Discover{client:client, discoverStates:discoverStates{states:make(map[string]*DiscoverState)}}
	d.defaultHandlers.AddOnPutHandler(DefaultOnPutHandler)
	d.defaultHandlers.AddOnDelHandlers(DefaultOnDelHandler)
	d.defaultHandlers.AddOnDoneHandler(DefaultOnDoneHandler)
	d.defaultHandlers.AddOnFailedHandler(DefaultOnFailedHandler)
	return &d
}

func NewDiscover(client *clientv3.Client) *Discover {
	d := Discover{client:client, discoverStates:discoverStates{states:make(map[string]*DiscoverState)}}
	return &d
}