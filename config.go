package gosensus

import "go.uber.org/zap"

type Config struct {
	EtcdEndpoints []string
	Logger 		  zap.Logger
	DataDir 	  string
}
