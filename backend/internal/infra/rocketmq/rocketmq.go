package rocketmq

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/mq"
)

type Producer struct {
	client rocketmq.Producer
}

func Connect(ctx context.Context, cfg config.RocketMQ) (*Producer, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// RocketMQ Go 客户端接收 IP 地址，先解析 Compose 服务名。
	addresses, err := resolveNameServers(ctx, cfg.NameServers)
	if err != nil {
		return nil, err
	}
	client, err := rocketmq.NewProducer(
		producer.WithNameServer(addresses),
		producer.WithGroupName("simple_bilibili_transcode_producer"),
		producer.WithSendMsgTimeout(3*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化 RocketMQ 生产者: %w", err)
	}
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("启动 RocketMQ 生产者: %w", err)
	}
	return &Producer{client: client}, nil
}

func resolveNameServers(ctx context.Context, hosts []string) (primitive.NamesrvAddr, error) {
	addresses := make(primitive.NamesrvAddr, 0, len(hosts))
	for _, endpoint := range hosts {
		host, port, err := net.SplitHostPort(endpoint)
		if err != nil {
			return nil, fmt.Errorf("解析 RocketMQ NameServer 地址 %q: %w", endpoint, err)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("解析 RocketMQ NameServer 主机 %q: %w", host, err)
		}
		for _, ip := range ips {
			addresses = append(addresses, net.JoinHostPort(ip.String(), port))
		}
	}
	return addresses, nil
}

func (p *Producer) SendVideoTranscode(ctx context.Context, task mq.VideoTranscodeTask) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("编码转码任务: %w", err)
	}
	msg := primitive.NewMessage(mq.VideoTranscodeTopic, body).WithKeys([]string{strconv.FormatInt(task.TaskID, 10)})
	result, err := p.client.SendSync(ctx, msg)
	if err != nil {
		return fmt.Errorf("发送转码任务: %w", err)
	}
	if result.Status != primitive.SendOK {
		return fmt.Errorf("RocketMQ 未确认转码任务，状态 %v", result.Status)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.client.Shutdown()
}
