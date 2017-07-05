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

## Set the CAS_URL environment variable in the dc.sh script

SSIA

# Start up the app

In the same directory as this README, run the following:

    ./dc.sh up -d

# Post a configuration to the running nginx-app-configurer

    curl -d '{"identifier":"1","url":"http://proxy:8080"}' http://<your-ip>:9091/api/

# Hit http://<your-ip>/apps/1 in a browser.

The Jupyter Notebook UI should start up.
