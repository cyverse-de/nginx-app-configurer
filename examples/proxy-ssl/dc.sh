#!/bin/bash

export BASE_URL_PATH=/apps/1
export ORIGIN_HOST=192.168.1.1
export ORIGIN_PORT=80
export CAS_URL=http://example-cas/cas/
export HOST_NGINX_SSL_KEY_PATH=
export HOST_NGINX_SSL_CERT_PATH=
export NGINX_SSL_KEY_PATH=
export NGINX_SSL_CERT_PATH
export HOST_PROXY_SSL_KEY_PATH=
export HOST_PROXY_SSL_CERT_PATH=
export PROXY_SSL_KEY_PATH=
export PROXY_SSL_CERT_PATH=
export HOST_NGINX_CA_CERT_PATH=
export NGINX_CA_CERT_PATH=/etc/ssl/trusted.crt

docker-compose -f docker-compose.yml "$@"
