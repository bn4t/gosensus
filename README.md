# Gosensus
![Tests](https://github.com/onclick-IT/gosensus/workflows/CI/badge.svg?branch=master)

This package implements a simple consensus algorithm to elect a leader among a number of nodes.

## Installation

```
go get github.com/onclick-IT/gosensus
```

## How to use

```go
package main

import (
    "github.com/onclick-IT/gosensus"
    "go.etcd.io/etcd/v3/clientv3"
    "go.uber.org/zap"
    "log"
    "time"
)

func main() { 
        // init etcd client 
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	
        // init zap logger
	Zap, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}

        // init gosensus client
	GosClient := &gosensus.Client{
		EtcdClient: etcdCli,
		Logger:     Zap,
		DataDir:    "/var/lib/gosensus/",
	}
	if err := GosClient.Start(); err != nil {
		log.Fatal("failed to start consensus", zap.Error(err))
	}
	
        log.Print(GosClient.IsLeader())
	
        GosClient.Stop()
}

```

## License
TBD