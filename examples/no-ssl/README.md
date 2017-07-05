Example without SSL
===================

This example demonstrates how to run nginx-app-configurer locally without SSL
configured.

# Preparations

## Install Docker

## Install docker-compose

## Build the nginx-app-configurer Docker container

In the top-level directory of this project, run the following:

    docker build --rm -t discoenv/nginx-app-configurer .

## Set your IP address in the dc.sh script

Find your local ip address and replace the value for the ORIGIN_HOST environment
variable in the dc.sh script.

# Start up the app

In the same directory as this README, run the following:

    ./dc.sh up -d

# Post a configuration to the running nginx-app-configurer

    curl -d '{"identifier":"1","url":"http://notebook:8888"}' http://localhost:9091/api/

# Hit http://localhost/apps/1 in a browser.

The Jupyter Notebook UI should start up.
