package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"

	"os"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func WriteHWInfo(out io.Writer) error {
	b, err := json.Marshal(GetMachineInfo())
	if err != nil {
		return err
	}
	err = binary.Write(out, binary.LittleEndian, uint64(len(b)))
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}

func SendHashListEntry(pipe io.ReadWriter, path string, hash []byte) (bool, error) {
	err := binary.Write(pipe, binary.LittleEndian, hash)
	if err != nil {
		return false, err
	}
	bytesPath := []byte(path)
	err = binary.Write(pipe, binary.LittleEndian, uint64(len(bytesPath)))
	if err != nil {
		return false, err
	}
	err = binary.Write(pipe, binary.LittleEndian, bytesPath)
	if err != nil {
		return false, err
	}
	resp := false
	err = binary.Read(pipe, binary.LittleEndian, &resp)
	return resp, err
}

func FinishHashList(pipe io.ReadWriter) error {
	zeroes := [32]byte{}
	_, err := pipe.Write(zeroes[:])
	return err
}

type Packet struct {
	Hash      [32]byte
	Signature [112]byte
	FilePath  string
	Blob      []byte
}

func ReadPacket(in io.Reader) (*Packet, error) {
	res := new(Packet)
	err := binary.Read(in, binary.LittleEndian, &res.Hash)
	if err != nil {
		return nil, err
	}
	err = binary.Read(in, binary.LittleEndian, &res.Signature)
	if err != nil {
		return nil, err
	}
	var size uint64
	err = binary.Read(in, binary.LittleEndian, &size)
	if err != nil {
		return nil, err
	}
	pathBytes := make([]byte, size)
	err = binary.Read(in, binary.LittleEndian, pathBytes)
	if err != nil {
		return nil, err
	}
	res.FilePath = string(pathBytes)
	size = uint64(0)
	err = binary.Read(in, binary.LittleEndian, &size)
	if err != nil {
		return nil, err
	}
	res.Blob = make([]byte, size)
	err = binary.Read(in, binary.LittleEndian, &res.Blob)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (p Packet) Verify() bool {
	sum := sha256.Sum256(p.Blob)
	if sum != p.Hash {
		return false
	}
	// crypto.go
	isValid := Verify(sum[:], p.Signature)
	return isValid
}
