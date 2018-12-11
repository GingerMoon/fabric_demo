package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"github.com/hyperledger/fabric/bccsp/utils"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/chaincode/shim/ext/entities"
	"github.com/pkg/errors"
)

// the functions below show some best practices on how
// to use entities to perform cryptographic operations
// over the ledger state

// getStateAndDecrypt retrieves the value associated to key,
// decrypts it with the supplied entity and returns the result
// of the decryption
func getStateAndDecrypt(stub shim.ChaincodeStubInterface, ent entities.Encrypter, key string) ([]byte, error) {
	// at first we retrieve the ciphertext from the ledger
	ciphertext, err := stub.GetState(key)
	if err != nil {
		return nil, err
	}

	// GetState will return a nil slice if the key does not exist.
	// Note that the chaincode logic may want to distinguish between
	// nil slice (key doesn't exist in state db) and empty slice
	// (key found in state db but value is empty). We do not
	// distinguish the case here
	if len(ciphertext) == 0 {
		return nil, errors.New("no ciphertext to decrypt")
	}

	state, err := ent.Decrypt(ciphertext)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return state, nil
}

// encryptAndPutState encrypts the supplied value using the
// supplied entity and puts it to the ledger associated to
// the supplied KVS key
func encryptAndPutState(stub shim.ChaincodeStubInterface, ent entities.Encrypter, key string, value []byte) error {

	// at first we use the supplied entity to encrypt the value
	ciphertext, err := ent.Encrypt(value)
	if err != nil {
		return err
	}

	return stub.PutState(key, ciphertext)
}

func parseEcdsaPubkey(b []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.WithStack(errors.New(fmt.Sprintf("pem.Decode(ecdsakey) failed. %+v", b)))
	}
	genericPublicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.WithStack(errors.WithMessage(err, fmt.Sprintf("x509.ParseECPrivateKey(block.Bytes) failed. %+v", block.Bytes)))
	}
	pubkey := genericPublicKey.(*ecdsa.PublicKey)
	return pubkey, nil
}

func verifyECDSA(pubkey *ecdsa.PublicKey, signature, digest string) (bool, error) {
	logger.Infof("verifyECDSA (signature): %s", signature)
	logger.Infof("verifyECDSA (digest): %s", digest)

	sigbytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, errors.WithStack(err)
	}

	r, s, err := utils.UnmarshalECDSASignature(sigbytes)
	if err != nil {
		return false, errors.WithStack(fmt.Errorf("Failed unmashalling signature [%s]", err))
	}

	lowS, err := utils.IsLowS(pubkey, s)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if !lowS {
		return false, errors.WithStack(fmt.Errorf("Invalid S. Must be smaller than half the order [%s][%s].", s, utils.GetCurveHalfOrdersAt(pubkey.Curve)))
	}
	hash := sha256.Sum256([]byte(digest))
	return ecdsa.Verify(pubkey, hash[:], r, s), nil
}