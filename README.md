# demo

Notes:
1. Please modify the demo/fabric-network/first-network/docker-compose-cli.yaml to deploy the service on the specified host.
 constraints: [node.hostname == bingsu-sw11]
2. The demo respository need to be located under /home/docker/ on every host.
3. Please make sure the system is clean (ESPECIALLY no existing volume, no existing chaincode image ) before you start the Fabric network.

Steps:
1. Build swarm cluster
docker swarm init
copy the command "docker join ...." in the returned resut to join the swarm master.

2. Clear the existing Fabric network (if exists)
cd /home/docker/demo/fabric-network/first-network
./debug stop
./debug prune
./debug ls
docker image rm ... # remove the existing chaincode images which looks like dev-xxxxx

3. Build the Fabric network
cd /home/docker/demo/fabric-network/first-network
./debug start
docker exec -it fabric_clixxxxxxxxx bash # enter into the cli container

4. cd ./scripts

5. ./createFabricNetwork.sh


