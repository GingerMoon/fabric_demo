



# Accelor Demo Application Spec

### Versions
10/27/2018, Xiaohan Ma, v1: Init the spec writing

### Overview
Three demo applications need for showcase of our Accelor chip family:

•  Payment
•  E-bill
•  Logistics


### Payment
Test application (payment) shares the following features:

1. High concurrency: at least 1000 users/peers.
2. AES is used for encryption/decryption. ECDSA used for sign(client side) and verify(chaincode side).
3. Simple transactions randomly involved among users.
4. Place a big ledger chunk – size > 1GB.
5. Support 3 physical nodes.

### Test Steps
1. Start the Fabric-network (forked from the [BYFN](https://hyperledger-fabric.readthedocs.io/en/latest/build_network.html) sample)  with the chain code also deployed.
   - export PATH=$GOPATH/sw/accelor-demo/fabric-network/bin:$PATH
   - cd fabric-network/first-network
   - ./byfn.sh generate
   - ./byfn.sh up

2. Start the client in the running cli container.
   - copy the vendor folders manually into accelor-demo and the chaincode folder.
   - docker exec -it cli bash
   - cd accelor-demo/
   - ./start.sh

#### Notes:
- We are using Fabric 1.3 (./fabric-network/bin).
- ECDSA sign cannot be used in chaincode because it is not deterministic and can cause endorsement error.
- If you meet errors about dep, please move the chaincode fold outside of the accelor-demo folder.
- Because dep init doesn't work in China, so we need to copy the vendor tars for the client (vendor.tar.gz) and chaincode (vendor.cc.tar.gz) manually.
- in the path of accelor-demo:
- tar -xvzf vendor.tar.gz
- cd accelor-demo/fabric-network/chaincode_example02/go
- tar -xvzf vendor.cc.tar.gz