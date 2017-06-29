#!/bin/bash

export BASE_URL_PATH=/apps/1
export ORIGIN_HOST=192.168.1.11
export ORIGIN_PORT=80

docker-compose -f docker-compose.yml "$@"
