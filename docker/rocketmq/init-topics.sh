#!/bin/sh
set -eu

NAMESRV_ADDR="${NAMESRV_ADDR:-rmqnamesrv:9876}"
CLUSTER_NAME="${CLUSTER_NAME:-DefaultCluster}"
TOPICS="video_transcode_topic video_stat_sync_topic danmaku_persist_topic user_behavior_topic watch_history_sync_topic dynamic_fanout_topic notification_topic"

echo "等待 RocketMQ Broker 注册到 NameServer (${NAMESRV_ADDR})..."
attempt=0
until sh mqadmin clusterList -n "${NAMESRV_ADDR}" 2>/dev/null | grep -q "${CLUSTER_NAME}"; do
  attempt=$((attempt + 1))
  if [ "${attempt}" -ge 30 ]; then
    echo "等待 RocketMQ Broker 就绪超时" >&2
    exit 1
  fi
  sleep 2
done

for topic in ${TOPICS}; do
  echo "创建或更新 Topic: ${topic}"
  sh mqadmin updateTopic \
    -n "${NAMESRV_ADDR}" \
    -c "${CLUSTER_NAME}" \
    -t "${topic}" \
    -r 4 \
    -w 4
done

echo "当前已创建 Topic 列表:"
sh mqadmin topicList -n "${NAMESRV_ADDR}"
