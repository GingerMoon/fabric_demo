package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"github.com/hyperledger/fabric/bccsp/utils"
	"github.com/pkg/errors"
)

func parseEcdsaPrikey(b []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.WithStack(errors.New(fmt.Sprintf("pem.Decode(ecdsakey) failed. %+v", b)))
	}
	prikey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.WithStack(errors.WithMessage(err, fmt.Sprintf("x509.ParseECPrivateKey(block.Bytes) failed. %+v", b)))
	}
	return prikey, nil
}

func sign(payload []byte, prikey *ecdsa.PrivateKey) (string, error) {
	pubkey := prikey.PublicKey

	hash := sha256.Sum256([]byte(payload))
	r, s, err := ecdsa.Sign(rand.Reader, prikey, hash[:])
	if err != nil {
		return "", errors.WithStack(err)
	}

	s, _, err = utils.ToLowS(&pubkey, s)
	if err != nil {
		return "", errors.WithStack(err)
	}
	sigbytes, err := utils.MarshalECDSASignature(r, s)

	return base64.StdEncoding.EncodeToString(sigbytes), err
}