package gosensus

import (
	"go.etcd.io/etcd/v3/clientv3"
	"time"
)

var etcdClient *clientv3.Client

// Connect connects to the etcd cluster
func initEtcdClient(endpoints []string) (err error) {
	etcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	return
}
