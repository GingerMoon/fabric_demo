# Using payment_cc

To test `payment_cc` you need to first generate an AES 256 bit key as a base64
encoded string so that it can be passed as JSON to the peer chaincode 
invoke's transient parameter.

Note: Before getting started you must use [dep](https://golang.github.io/dep/) to add external dependencies.  Please issue the following commands inside the folder of payment_cc.go:
```
dep init
dep ensure
```