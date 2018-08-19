// crypto.go - hardcoded keys and data signature verification
// Copyright (c) 2018  Hexawolf
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
// of the Software, and to permit persons to whom the Software is furnished to do
// so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
package main

import (
	"encoding/base64"

	"crypto/tls"
	"crypto/x509"

	"github.com/twstrike/ed448"
	"io/ioutil"
	"crypto/rand"
	"strings"
)

var publicKey [56]byte
var curve ed448.DecafCurve
var conf tls.Config

// Both variables are set by build script.
var certEnc, keyEnc string

func LoadKeys() error {
	publicKeySlice, pubErr := base64.StdEncoding.DecodeString(keyEnc)
	if pubErr != nil {
		return pubErr
	}
	copy(publicKey[:], publicKeySlice)
	curve = ed448.NewDecafCurve()

	certs := x509.NewCertPool()
	cert := "-----BEGIN CERTIFICATE-----\n" + certEnc + "\n-----END CERTIFICATE-----"
	certs.AppendCertsFromPEM([]byte(cert))
	conf = tls.Config{
		RootCAs:    certs,
		// Extract domain from targetHost
		ServerName: strings.Split(targetHost, ":")[0],
	}
	return nil
}

func Verify(data []byte, signature [112]byte) bool {
	verify, err := curve.Verify(signature, data, publicKey)
	return verify && err == nil
}

func newUUID() ([]byte, error) {
	v := make([]byte, 32)
	_, err := rand.Read(v)
	return v, err
}

func UUID() ([]byte, error) {
	uuidLocation := "config/uuid.bin"
	if fileExists(uuidLocation) {
		return ioutil.ReadFile(uuidLocation)
	} else {
		b, err := newUUID()
		if err != nil {
			return nil, err
		}
		ioutil.WriteFile(uuidLocation, b, 0600)
		return b, nil
	}
}
