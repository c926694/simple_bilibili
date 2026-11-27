package rocketmq

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/rlog"

	"simple_bilibili/backend/internal/config"
)

type MessageModel = consumer.MessageModel
type ConsumeFromWhere = consumer.ConsumeFromWhere

const (
	Clustering             = consumer.Clustering
	Broadcasting           = consumer.BroadCasting
	ConsumeFromLastOffset  = consumer.ConsumeFromLastOffset
	ConsumeFromFirstOffset = consumer.ConsumeFromFirstOffset
	ConsumeFromTimestamp   = consumer.ConsumeFromTimestamp
)

type PushHandler func(ctx context.Context, msg Message) error

type PushConsumer struct {
	client        rocketmq.PushConsumer
	handleTimeout time.Duration
	mu            sync.Mutex
	started       bool
	startErr      error
	closed        bool
}

type pushSettings struct {
	maxReconsume  int32
	handleTimeout time.Duration
	messageModel  MessageModel
	fromWhere     ConsumeFromWhere
}

type PushOption func(*pushSettings)

func WithMaxReconsumeTimes(times int32) PushOption {
	return func(settings *pushSettings) { settings.maxReconsume = times }
}

func WithHandleTimeout(timeout time.Duration) PushOption {
	return func(settings *pushSettings) { settings.handleTimeout = timeout }
}

func WithMessageModel(model MessageModel) PushOption {
	return func(settings *pushSettings) { settings.messageModel = model }
}

func WithConsumeFromWhere(from ConsumeFromWhere) PushOption {
	return func(settings *pushSettings) { settings.fromWhere = from }
}

func NewPushConsumer(ctx context.Context, cfg config.RocketMQ, groupName string, opts ...PushOption) (*PushConsumer, error) {
	settings := pushSettings{
		maxReconsume: -1, handleTimeout: 30 * time.Second,
		messageModel: Clustering, fromWhere: ConsumeFromLastOffset,
	}
	for _, opt := range opts {
		opt(&settings)
	}
	if strings.TrimSpace(groupName) == "" || settings.maxReconsume < -1 || settings.handleTimeout <= 0 {
		return nil, errors.New("RocketMQ 推消费者组名、重试次数或处理超时无效")
	}
	if (settings.messageModel != Clustering && settings.messageModel != Broadcasting) ||
		settings.fromWhere < ConsumeFromLastOffset || settings.fromWhere > ConsumeFromTimestamp {
		return nil, errors.New("RocketMQ 推消费者的消息模式或起始位点设置无效")
	}
	addresses, err := resolveNameServers(ctx, cfg.NameServers)
	if err != nil {
		return nil, err
	}
	client, err := rocketmq.NewPushConsumer(
		consumer.WithNameServer(addresses),
		consumer.WithGroupName(groupName),
		consumer.WithInstance("push-"+rand.Text()),
		consumer.WithMaxReconsumeTimes(settings.maxReconsume),
		// 封装负责处理期限与重投，底层不再独立扫描并重投同一条消息。
		consumer.WithConsumeTimeout(time.Duration(math.MaxInt64)),
		consumer.WithConsumerModel(settings.messageModel),
		consumer.WithConsumeFromWhere(settings.fromWhere),
		consumer.WithConsumeMessageBatchMaxSize(1),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化 RocketMQ 推消费者: %w", err)
	}
	return &PushConsumer{client: client, handleTimeout: settings.handleTimeout}, nil
}

func (c *PushConsumer) Subscribe(topic, tagExpr string, handler PushHandler) error {
	if strings.TrimSpace(topic) == "" || handler == nil {
		return errors.New("RocketMQ Topic 和推消息处理函数不能为空")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started || c.closed {
		return errors.New("RocketMQ 推消费者须在启动前订阅")
	}
	selector := consumer.MessageSelector{Type: consumer.TAG, Expression: subscription(tagExpr)}
	err := c.client.Subscribe(topic, selector, func(ctx context.Context, messages ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, message := range messages {
			msg := fromSDK(message)
			if err := c.handle(ctx, handler, msg); err != nil {
				rlog.Warning("处理 RocketMQ 推消息失败", map[string]any{"msg_id": msg.MsgID, "topic": msg.Topic, "error": err.Error()})
				if !IsPermanentError(err) {
					return consumer.ConsumeRetryLater, nil
				}
			}
		}
		return consumer.ConsumeSuccess, nil
	})
	if err != nil {
		return fmt.Errorf("订阅 RocketMQ Topic %q: %w", topic, err)
	}
	return nil
}

func (c *PushConsumer) handle(ctx context.Context, handler PushHandler, msg Message) (err error) {
	ctx, cancel := context.WithTimeout(WithTraceID(ctx, msg.Properties[TraceIDProperty]), c.handleTimeout)
	defer cancel()
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("处理 RocketMQ 推消息时发生 panic: %v", value)
		}
	}()

	if err := handler(ctx, msg); err != nil {
		return err
	}
	return ctx.Err()
}

func (c *PushConsumer) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("RocketMQ 推消费者已关闭")
	}
	if !c.started {
		c.started = true
		if err := c.client.Start(); err != nil {
			c.startErr = fmt.Errorf("启动 RocketMQ 推消费者: %w", err)
		}
	}
	return c.startErr
}

func (c *PushConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if !c.started || c.startErr != nil {
		return nil
	}
	return c.client.Shutdown()
}
