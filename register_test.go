package guard

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"testing"
	"time"
)

var client,_ = clientv3.New(clientv3.Config{
	Endpoints:   []string{"127.0.0.1:2379"},
	DialTimeout: 2 * time.Second,
})

func TestRegister_Append(t *testing.T) {
	key := "abc"
	r := NewDefaultRegister(client)
	_ = r.Append(&RegisterOption{Path: key, Info: DataContainer{HostName: "aaaa"}, Multi: true})
	resp, err := client.Get(context.TODO(), key, clientv3.WithPrefix())
	if err != nil || resp.Count != 1{
		t.Error("Register error")
	}
	err = r.Append(&RegisterOption{Path:"abc", Info: DataContainer{HostName:"aaaa"}, Multi:true})
	if err == nil{
		t.Error("Repeated registration")
	}
	time.Sleep(time.Second*1)
	r.Stop("abc")
	time.Sleep(time.Second*3)
	resp, err = client.Get(context.TODO(), key, clientv3.WithPrefix())
	if err != nil || resp.Count != 0{
		t.Error("Stop error")
	}
	time.Sleep(time.Second*3)
}

