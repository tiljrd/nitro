#!/usr/bin/env bash
set -euo pipefail

JWT_FILE=${JWT_FILE:-/jwtsecret}
if [ ! -f "$JWT_FILE" ]; then
  head -c32 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$JWT_FILE"
fi

ARBRETH_ARGS=${ARBRETH_ARGS:-}
ARBRETH_HTTP_ADDR=${ARBRETH_HTTP_ADDR:-0.0.0.0}
ARBRETH_HTTP_PORT=${ARBRETH_HTTP_PORT:-8547}
ARBRETH_AUTH_ADDR=${ARBRETH_AUTH_ADDR:-0.0.0.0}
ARBRETH_AUTH_PORT=${ARBRETH_AUTH_PORT:-8551}

NITRO_ARGS=${NITRO_ARGS:-}
EXECUTION_BACKEND=${EXECUTION_BACKEND:-reth}
RETH_URL=${RETH_URL:-http://127.0.0.1:${ARBRETH_HTTP_PORT}}

set -x
arb-reth \
  --http.addr "${ARBRETH_HTTP_ADDR}" \
  --http.port "${ARBRETH_HTTP_PORT}" \
  --auth.addr "${ARBRETH_AUTH_ADDR}" \
  --auth.port "${ARBRETH_AUTH_PORT}" \
  --auth.jwtsecret "${JWT_FILE}" \
  ${ARBRETH_ARGS} &
ARBRETH_PID=$!

sleep 2

nitro \
  --execution-backend "${EXECUTION_BACKEND}" \
  --execution-reth.url "${RETH_URL}" \
  --execution-reth.jwt-secret-path "${JWT_FILE}" \
  ${NITRO_ARGS} &
NITRO_PID=$!

trap "kill -TERM ${NITRO_PID} ${ARBRETH_PID}; wait" SIGINT SIGTERM
wait
