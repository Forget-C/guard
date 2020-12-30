package guard

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"testing"
	"time"
)

func TestDiscover_Append(t *testing.T) {
	key := "abc"
	d := NewDefaultDiscover(client)
	err := d.Append(&DiscoverOption{Path:key, Prefix:true})
	if err !=nil{
		t.Error(err)
	}
	time.Sleep(time.Second*1)
	r := NewDefaultRegister(client)
	_ = r.Append(&RegisterOption{Path: key, Info: DataContainer{HostName: "aaaa"}, Multi: true})
	resp, err := client.Get(context.TODO(), key, clientv3.WithPrefix())
	if err != nil || resp.Count != 1{
		t.Error("Register error")
	}
	time.Sleep(time.Second*1)
	if _, exist := d.states[key]; !exist || d.states[key].children.Count() == 0{
		t.Error("Discover error")
	}
	t.Log(d.states,d.states[key].Data, d.states[key].children)
	r.StopAll()
	time.Sleep(time.Second*3)
	if _, exist := d.states[key]; !exist ||  d.states[key].children.Count() != 0{
		t.Error("Discover error")
	}
	t.Log(d.states,d.states[key].Data, d.states[key].children)
}

func TestDiscoverStates_PrefixRoundRobinChildren(t *testing.T) {
	key := "abc"
	d := NewDefaultDiscover(client)
	err := d.Append(&DiscoverOption{Path:key, Prefix:true})
	if err !=nil{
		t.Error(err)
	}
	time.Sleep(time.Second*1)
	r := NewDefaultRegister(client)
	_ = r.Append(&RegisterOption{Path: key, Info: DataContainer{HostName: "aaaa"}, Multi: true})
	resp, err := client.Get(context.TODO(), key, clientv3.WithPrefix())
	if err != nil || resp.Count != 1{
		t.Error("Register error")
	}
	// 不允许一个进程相同的path注册两次， 所以加上了"/12345"
	_ = r.Append(&RegisterOption{Path: key+"/12345", Info: DataContainer{HostName: "aaaa"}, Multi: true})
	resp, err = client.Get(context.TODO(), key, clientv3.WithPrefix())
	if err != nil || resp.Count !=2 {
		t.Error("Register error", err)
	}
	s1 := d.PrefixRoundRobinChildren(key)
	t.Log(s1)
	s2 := d.PrefixRoundRobinChildren(key)
	t.Log(s2)
	if s1.CurrentPath == s2.CurrentPath {
		t.Error("Polling failure")
	}
}