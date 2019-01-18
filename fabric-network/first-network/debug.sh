#!/bin/bash
# ./byfn.sh down
# ./byfn.sh generate
# ./byfn.sh up


function stop() {
    docker stack rm fabric
}

function prune() {
    yes y | docker container prune
    yes y | docker image prune
    yes y | docker volume prune
    yes y | docker network prune
    clear
} 

function ls() {
    echo "container ****************"
    docker container ls -a
    echo "volume ****************"
    docker volume ls
    echo "network ****************"
    docker network ls
    echo "image ****************"
    docker image ls -a
} 

function start() {
    docker stack deploy --compose-file docker-compose-cli.yaml fabric
} 


MODE=$1

if [ "${MODE}" == "stop" ]; then
  stop
elif [ "${MODE}" == "prune" ]; then ## Clear the network
  prune
elif [ "${MODE}" == "ls" ]; then ## Generate Artifacts
  ls
elif [ "${MODE}" == "start" ]; then ## Restart the network
  docker stack deploy --compose-file docker-compose-cli.yaml fabric
else
  exit 1
fi
