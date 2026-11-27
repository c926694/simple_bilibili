package rocketmq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apache/rocketmq-client-go/v2/primitive"
)

const TraceIDProperty = "trace_id"

type Message struct {
	MsgID          string
	Topic          string
	Tag            string
	Keys           []string
	ReconsumeTimes int32
	Properties     map[string]string
	Body           []byte
}

func (m Message) DecodeJSON(dst any) error {
	if err := json.Unmarshal(m.Body, dst); err != nil {
		return fmt.Errorf("解析 RocketMQ 消息 JSON: %w", err)
	}
	return nil
}

type traceIDKey struct{}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(traceIDKey{}).(string)
	return traceID
}

func fromSDK(msg *primitive.MessageExt) Message {
	return Message{
		MsgID: msg.MsgId, Topic: msg.Topic, Tag: msg.GetTags(), Keys: strings.Fields(msg.GetKeys()),
		ReconsumeTimes: msg.ReconsumeTimes, Properties: msg.GetProperties(), Body: bytes.Clone(msg.Body),
	}
}

func subscription(tagExpr string) string {
	if tagExpr == "" {
		return "*"
	}
	return tagExpr
}
