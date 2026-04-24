#!/bin/sh
# Redis seed script for the simpanan examples.
# Run by the redis-seed sidecar in docker-compose.yaml.
#
# Each command runs as a separate redis-cli invocation so the shell
# (not redis-cli's pipe mode) handles quoting. Cleaner than piping a
# script of mixed comment + quoted-JSON lines through redis-cli stdin.

set -e
HOST="${REDIS_HOST:-redis}"
RC="redis-cli -h $HOST"

# String keys ----------------------------------------------------
$RC SET user:42         '{"id":42,"email":"fox@example.com","name":"Fox","role":"member"}'
$RC SET user:42:profile '{"avatar":"https://cdn.example.com/avatars/42.png","bio":"loves espresso"}'
$RC SET ratelimit:user:1337 42

# A few extra string keys so KEYS / SCAN return more than the example set.
$RC SET cache:product:1 '{"name":"Standard espresso","price_cents":300}'
$RC SET cache:product:2 '{"name":"Cappuccino","price_cents":450}'
$RC SET cache:product:4 '{"name":"Branded mug","price_cents":1800}'

# Hash keys ------------------------------------------------------
$RC HSET session:abc123 user_id 1 created_at 1714000000
$RC HSET session:def456 user_id 3 created_at 1714003600

echo "redis seeded: $($RC DBSIZE) keys total"
