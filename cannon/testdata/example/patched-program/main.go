package main

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed placeholder.bin
var placeholder []byte

type VMData struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

func main() {
	_, _ = os.Stdout.Write([]byte("hello world!\n"))

	datalen := binary.BigEndian.Uint64(placeholder)
	configBuf := placeholder[8 : 8+datalen]
	var vmData VMData
	err := json.Unmarshal(configBuf, &vmData)
	if err != nil {
		panic(fmt.Sprintf("failed to decode vm data %v", err))
	}
	fmt.Printf("vm data: %#v\n", vmData)
	if vmData.Name != "cannon-load" {
		panic("invalid vm name")
	}
	if vmData.Version != 2 {
		panic("invalid vm version")
	}
}
