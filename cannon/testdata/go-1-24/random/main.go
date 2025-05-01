package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
)

func main() {
	var buf = make([]byte, 8)
	var randomInt int64
	if _, err := crand.Read(buf); err == nil {
		randomInt = int64(binary.BigEndian.Uint64(buf))
	}
	fmt.Printf("Random int: %d\n", randomInt)
}
