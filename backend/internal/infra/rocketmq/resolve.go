package rocketmq

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/rlog"
)

func init() {
	rlog.SetLogLevel("warn")
}

func resolveNameServers(ctx context.Context, hosts []string) (primitive.NamesrvAddr, error) {
	if len(hosts) == 0 {
		return nil, errors.New("RocketMQ NameServer 地址不能为空")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	addresses := make(primitive.NamesrvAddr, 0, len(hosts))
	for _, endpoint := range hosts {
		host, port, err := net.SplitHostPort(endpoint)
		if err != nil {
			return nil, fmt.Errorf("解析 RocketMQ NameServer 地址 %q: %w", endpoint, err)
		}
		// RocketMQ Go 客户端接收 IP 地址，Compose 服务名由 DNS 解析。
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("解析 RocketMQ NameServer 主机 %q: %w", host, err)
		}
		for _, ip := range ips {
			addresses = append(addresses, net.JoinHostPort(ip.String(), port))
		}
	}
	if err := addresses.Check(); err != nil {
		return nil, fmt.Errorf("校验 RocketMQ NameServer 地址: %w", err)
	}
	return addresses, nil
}

type permanentError struct {
	cause error
}

func (e *permanentError) Error() string { return e.cause.Error() }
func (e *permanentError) Unwrap() error { return e.cause }

func PermanentError(err error) error {
	if err == nil || IsPermanentError(err) {
		return err
	}
	return &permanentError{cause: err}
}

func IsPermanentError(err error) bool {
	var target *permanentError
	return errors.As(err, &target)
}
