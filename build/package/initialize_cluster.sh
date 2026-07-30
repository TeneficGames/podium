#!/bin/bash

HOSTS=$(echo $CLUSTER_HOSTS | tr " " "\n")
HOST_PORTS=()
for element in $HOSTS; do
  HOST_NAME=$(echo $element | cut -d ":" -f 1)
  PORT=$(echo $element | cut -d ":" -f 2)
  for attempt in $(seq 1 50); do
    if redis-cli -h "${HOST_NAME}" -p "${PORT}" ping >/dev/null 2>&1; then
      break
    fi
    if [ "${attempt}" -eq 50 ]; then
      echo "Redis node ${HOST_NAME}:${PORT} did not become ready"
      exit 1
    fi
    sleep 0.1
  done
  IP=$(host ${HOST_NAME} | grep "has address" | cut -d " " -f 4)

  HOST_PORTS=("${HOST_PORTS[@]}" "$(echo $IP:$PORT)")
done

redis-cli --cluster create ${HOST_PORTS[@]} --cluster-yes
