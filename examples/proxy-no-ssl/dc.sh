#!/bin/bash

export BASE_URL_PATH=/apps/1
export ORIGIN_HOST=192.168.1.1
export ORIGIN_PORT=80
export CAS_URL=http://example-cas/cas/

docker-compose -f docker-compose.yml "$@"
