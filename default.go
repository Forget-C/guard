package guard

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/go-resty/resty/v2"
	"google.golang.org/grpc"
	"time"
)

// 状态接口
type State interface {
	AddOnPutHandler(handler ChangedHandler)
	AddOnDelHandlers(handler ChangedHandler)
	AddOnFailedHandler(handler FailedHandler)
	AddOnDoneHandler(handler DoneHandler)
	AddOnIntervalHandler(handler IntervalHandler)
	Start()
	Stop()
}

// 当增、删时触发的函数接口
type ChangedHandler func(key []byte, value []byte, state *DiscoverState)

// 当失败时触发的函数接口
type FailedHandler func(key []byte, value []byte, state State, err error)

// 当结束时触发的函数接口
type DoneHandler func(key []byte, state State)

// 当间隔时触发的函数接口
type IntervalHandler func(key []byte, state *RegisterState, KeepaliveResponse *clientv3.LeaseKeepAliveResponse)

// 默认的增加时触发的函数
func DefaultOnPutHandler(key []byte, value []byte, state *DiscoverState)  {
	var data DataContainer
	ks := string(key)
	if err := json.Unmarshal(value, &data); err != nil {
		for _, handler := range state.onFailedHandler{
			if handler == nil{
				continue
			}
			handler(key, value, state, err)
		}
	}
	if ks != state.Option.Path {
		childState := DiscoverState{
			Option:state.Option,
			ctx:state.ctx,
			client:state.client,
			cancel: state.cancel,
			CurrentPath:ks,
			IsChild:true,
			Data:&data,
			handlers: state.handlers,
		}
		state.children.add(ks, &childState)
	}else {
		state.Data = &data
	}
	return
}


// 默认的删除时触发的函数
func DefaultOnDelHandler(key []byte, value []byte, state *DiscoverState)  {
	ks := string(key)
	if ks != state.Option.Path {
		state.children.del(ks)
	}else {
		state.Data = nil
	}
	return
}

// 默认的失败时触发的函数
func DefaultOnFailedHandler(key []byte, value []byte, state State, err error)  {
	fmt.Println(err)
	return
}

// 默认的结束时触发的函数
func DefaultOnDoneHandler(key []byte, state State)  {
	return
}

// 默认的间隔时触发的函数
func DefaultOnIntervalHandler(key []byte, state *RegisterState, KeepaliveResponse *clientv3.LeaseKeepAliveResponse)  {
	return
}

// 触发函数集合
type handlers struct {
	onPutHandlers []ChangedHandler
	onDelHandlers []ChangedHandler
	onFailedHandler []FailedHandler
	onDoneHandler []DoneHandler
	onIntervalHandler []IntervalHandler
}

// 追加增加时触发的函数
func (h *handlers)AddOnPutHandler(handler ChangedHandler)  {
	h.onPutHandlers = append(h.onPutHandlers, handler)
}

// 追加删除时触发的函数
func (h *handlers)AddOnDelHandlers(handler ChangedHandler)  {
	h.onDelHandlers = append(h.onDelHandlers, handler)
}

// 追加失败时触发的函数
func (h *handlers)AddOnFailedHandler(handler FailedHandler)  {
	h.onFailedHandler = append(h.onFailedHandler, handler)
}

// 追加结束时触发的函数
func (h *handlers)AddOnDoneHandler(handler DoneHandler)  {
	h.onDoneHandler = append(h.onDoneHandler, handler)
}

// 追加间隔时触发的函数
func (h *handlers)AddOnIntervalHandler(handler IntervalHandler)  {
	h.onIntervalHandler = append(h.onIntervalHandler, handler)
}

// 数据容器
type DataContainer struct {
	ID        int64				// 唯一ID
	HostName  string			// 主机名
	GRPCPort  int				// GRPC端口
	Domain    string			// 主机坐在的域
	ServiceId int				// 服务ID
	HttpPort  int				// http端口
	MetaData  map[string]string // 扩展的信息
}

// 获取连接地址
func (d *DataContainer) getAddr() string {
	var addr string
	if len(d.Domain) == 0 {
		addr = fmt.Sprintf("%s", d.HostName)
	} else {
		addr = fmt.Sprintf("%s.%s", d.HostName, d.Domain)
	}
	return addr
}

//获取grpc连接
func (d *DataContainer) GetGRPCConn() (*grpc.ClientConn, error) {
	addr := d.getAddr()
	var conn *grpc.ClientConn
	var err error
	for i := 0; i < 3; i++ {
		conn, err = grpc.Dial(fmt.Sprintf(addr+":%d", d.GRPCPort), grpc.WithInsecure())
		if err == nil {
			break
		}
		time.Sleep(time.Second * 1)
	}
	return conn, err
}

// 获取http连接
func (d *DataContainer) GetHTTPConn() *resty.Client {
	addr := d.getAddr()
	var httpClient *resty.Client
	httpClient = resty.New()
	httpClient.SetRetryCount(3)
	httpClient.SetRetryWaitTime(time.Second * 3)
	httpClient.SetHostURL(fmt.Sprintf("http://"+addr+":%d", d.HttpPort))
	httpClient.SetHeaders(map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "Inner",
	})
	return httpClient
}