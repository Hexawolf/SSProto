// server.go - does all the SSProto magic ✨
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
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"io/ioutil"
	"log"
	"sync"
	"time"
)

// Client UUIDs seen since last reindexing. Used to filter repeated requests.
var seenIDs = make(map[string]struct{})
var seenIDsMtx sync.Mutex

func (s *Service) serve(conn *tls.Conn) {
	defer conn.Close()
	defer s.wg.Done()
	conn.SetDeadline(time.Now().Add(time.Second * 300))
	var size uint64

	// Protocol version
	{
		var pv uint8
		err := binary.Read(conn, binary.LittleEndian, &pv)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}
		binary.Write(conn, binary.LittleEndian, SSProtoVersion)
	}

	// Force pending reindexing if any so we will not
	// send newer version of file when we have only
	// hash of older version.
	if reindexRequired.IsSet() {
		filesMapLock.Lock()
		log.Println("Reindexing files...")
		ListFiles()
		seenIDsMtx.Lock()
		seenIDs = make(map[string]struct{})
		seenIDsMtx.Unlock()
		log.Println("Reindexing done")
		reindexTimer.Stop()
		reindexRequired.UnSet()
		filesMapLock.Unlock()
	}

	// Expecting 32-bytes long identifier
	data := make([]byte, 32)
	err := binary.Read(conn, binary.LittleEndian, data)
	if err != nil {
		log.Println("Stream error:", err)
		return
	}

	// Record machine data if it wasn't recorded yet
	baseEncodedID := base64.StdEncoding.EncodeToString(data)
	var machineData []byte

	if _, prs := seenIDs[baseEncodedID]; prs {
		log.Println("Rejecting connection - already served today.")
		err = binary.Write(conn, binary.LittleEndian, false)
		if err != nil {
			log.Println("Stream error:", err)
		}
		return
	}

	err = binary.Write(conn, binary.LittleEndian, true)
	if err != nil {
		log.Println("Stream error:", err)
		return
	}
	binary.Read(conn, binary.LittleEndian, &size)
	machineData = make([]byte, size)
	err = binary.Read(conn, binary.LittleEndian, machineData)
	if err != nil {
		log.Println("Stream error:", err)
		return
	}

	clientFiles := make(map[string]string)
	var clientList []string

	filesMapLock.RLock()
	defer filesMapLock.RUnlock()

	// Get hashes from client and create an intersection
	for {
		// Expect file hash
		var hash [32]byte
		err = binary.Read(conn, binary.LittleEndian, &hash)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}

		if bytes.Equal(hash[:], make([]byte, 32)) {
			break
		}

		// Expect size of file path string
		err = binary.Read(conn, binary.LittleEndian, &size)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}

		// Expect file path
		filePath := make([]byte, size)
		err = binary.Read(conn, binary.LittleEndian, filePath)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}

		// Construct client files list
		clientList = append(clientList, string(filePath))

		// Create intersection of client and server maps
		contains := false
		if v, ok := filesMap[string(filePath)]; ok {
			contains = bytes.Equal(v.Hash[:], hash[:])
			if contains {
				clientFiles[string(filePath)] = v.ServPath
			}
		}

		// Answer if file is valid
		err := binary.Write(conn, binary.LittleEndian, contains)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}
	}

	// Remove difference from server files to create a list of mods that we need to send
	changes := make(map[string]IndexedFile)
	for _, v := range filesMap {
		if _, ok := clientFiles[v.ClientPath]; ok {
			continue
		}
		changes[v.ClientPath] = v
	}

	for _, entry := range changes {
		skip := false
		for _, clientFile := range clientList {
			if clientFile == entry.ClientPath && entry.ShouldNotReplace {
				skip = true
			}
		}
		if skip {
			continue
		}

		// Read file to memory
		s, err := ioutil.ReadFile(entry.ServPath)
		if err != nil {
			log.Panicln("Failed to read file", entry.ServPath)
		}

		// Size of file path
		err = binary.Write(conn, binary.LittleEndian, uint64(len([]byte(entry.ClientPath))))
		if err != nil {
			log.Println("Stream error:", err)
		}

		// File path
		err = binary.Write(conn, binary.LittleEndian, []byte(entry.ClientPath))
		if err != nil {
			log.Println("Stream error:", err)
			return
		}

		// Size of file
		size = uint64(len(s))
		err = binary.Write(conn, binary.LittleEndian, size)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}

		// File blob
		err = binary.Write(conn, binary.LittleEndian, s)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}

	}

	seenIDsMtx.Lock()
	seenIDs[baseEncodedID] = struct{}{}
	seenIDsMtx.Unlock()
	// Logging virtual memory statistics received from the client to the log file
	log.Println("HWInfo:", baseEncodedID+": "+string(machineData))
	log.Println("Success!")
	s.wg.Done()
}
