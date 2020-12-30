package guard

import "github.com/coreos/etcd/clientv3"

func NewMonitor()  {

}

type Monitor struct {
	client *clientv3.Client

}
