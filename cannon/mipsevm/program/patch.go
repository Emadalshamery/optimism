package program

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/register"
	"github.com/ethereum/go-ethereum/common"
)

const WordSizeBytes = arch.WordSizeBytes

// PatchGoGC patches out garbage-collection-related symbols to disable garbage collection
// and improves performance by patching out floating-point-related symbols
func PatchGoGC(f *elf.File, st mipsevm.FPVMState) error {
	symbols, err := f.Symbols()
	if err != nil {
		return fmt.Errorf("failed to read symbols data, cannot patch program: %w", err)
	}

	for _, s := range symbols {
		// Disable Golang GC by patching the functions that enable the GC to a no-op function.
		switch s.Name {
		case "runtime.gcenable",
			"runtime.init.5",            // patch out: init() { go forcegchelper() }
			"runtime.main.func1",        // patch out: main.func() { newm(sysmon, ....) }
			"runtime.deductSweepCredit", // uses floating point nums and interacts with gc we disabled
			"runtime.(*gcControllerState).commit",
			// these prometheus packages rely on concurrent background things. We cannot run those.
			"github.com/prometheus/client_golang/prometheus.init",
			"github.com/prometheus/client_golang/prometheus.init.0",
			"github.com/prometheus/procfs.init",
			"github.com/prometheus/common/model.init",
			"github.com/prometheus/client_model/go.init",
			"github.com/prometheus/client_model/go.init.0",
			"github.com/prometheus/client_model/go.init.1",
			// skip flag pkg init, we need to debug arg-processing more to see why this fails
			"flag.init",
			// We need to patch this out, we don't pass float64nan because we don't support floats
			"runtime.check":
			// MIPSx patch: ret (pseudo instruction)
			// 03e00008 = jr $ra = ret (pseudo instruction)
			// 00000000 = nop (executes with delay-slot, but does nothing)
			if err := st.GetMemory().SetMemoryRange(Word(s.Value), bytes.NewReader([]byte{
				0x03, 0xe0, 0x00, 0x08,
				0, 0, 0, 0,
			})); err != nil {
				return fmt.Errorf("failed to patch Go runtime.gcenable: %w", err)
			}
		}
	}
	return nil
}

// PatchStack sets up the program's initial stack frame and stack pointer
func PatchStack(st mipsevm.FPVMState) error {
	// setup stack pointer
	sp := Word(arch.HighMemoryStart)
	// allocate 1 page for the initial stack data, and 16KB = 4 pages for the stack to grow
	if err := st.GetMemory().SetMemoryRange(sp-4*memory.PageSize, bytes.NewReader(make([]byte, 5*memory.PageSize))); err != nil {
		return errors.New("failed to allocate page for stack content")
	}
	st.GetRegistersRef()[register.RegSP] = sp

	storeMem := func(addr Word, v Word) {
		var dat [WordSizeBytes]byte
		arch.ByteOrderWord.PutWord(dat[:], v)
		_ = st.GetMemory().SetMemoryRange(addr, bytes.NewReader(dat[:]))
	}

	auxv3Offset := sp + WordSizeBytes*10
	randomness := []byte("4;byfairdiceroll")
	randomness = pad(randomness)
	_ = st.GetMemory().SetMemoryRange(auxv3Offset, bytes.NewReader(randomness))

	envp0Offset := auxv3Offset + Word(len(randomness))
	envar := append([]byte("GODEBUG=memprofilerate=0"), 0x0)
	envar = pad(envar)
	_ = st.GetMemory().SetMemoryRange(envp0Offset, bytes.NewReader(envar))

	argv0Offset := envp0Offset + Word(len(envar))
	programName := append([]byte("op-program"), 0x0)
	programName = pad(programName)
	_ = st.GetMemory().SetMemoryRange(argv0Offset, bytes.NewReader(programName))

	// init argc, argv, aux on stack
	storeMem(sp+WordSizeBytes*0, 1)           // argc = 1 (argument count)
	storeMem(sp+WordSizeBytes*1, argv0Offset) // argv[0]
	storeMem(sp+WordSizeBytes*2, 0)           // argv[1] = terminating
	storeMem(sp+WordSizeBytes*3, envp0Offset) // envp[0] = x (offset to first env var)
	storeMem(sp+WordSizeBytes*4, 0)           // envp[1] = terminating
	storeMem(sp+WordSizeBytes*5, 6)           // auxv[0] = _AT_PAGESZ = 6 (key)
	storeMem(sp+WordSizeBytes*6, 4096)        // auxv[1] = page size of 4 KiB (value) - (== minPhysPageSize)
	storeMem(sp+WordSizeBytes*7, 25)          // auxv[2] = AT_RANDOM
	storeMem(sp+WordSizeBytes*8, auxv3Offset) // auxv[3] = address of 16 bytes containing random value
	storeMem(sp+WordSizeBytes*9, 0)           // auxv[term] = 0

	return nil
}

const startMagicSize = 32

var targetMagic = common.Hex2Bytes("0e24fd47916aa3ac16264a6c874371fcdcc348505dac3ec4bb13df04c8c6f2e5")

func PatchVMData(st mipsevm.FPVMState, patch []byte) error {
	if st.GetStep() != 0 {
		return fmt.Errorf("invalid state")
	}

	mem := st.GetMemory()
	var currentMagic [startMagicSize]byte

	var addr Word
	// we know the Go compiler will place go:embeds in low memory rodata. So search ends at heap start
	for search := Word(0); search <= arch.HeapStart-startMagicSize; search += 8 {
		// assume 8-byte word for now
		v0 := mem.GetWord(search)
		v1 := mem.GetWord(search + 8)
		v2 := mem.GetWord(search + 16)
		v3 := mem.GetWord(search + 24)

		arch.ByteOrderWord.PutWord(currentMagic[0:8], v0)
		arch.ByteOrderWord.PutWord(currentMagic[8:16], v1)
		arch.ByteOrderWord.PutWord(currentMagic[16:24], v2)
		arch.ByteOrderWord.PutWord(currentMagic[24:32], v3)

		// Compare with target pattern
		if bytes.Equal(currentMagic[:], targetMagic[:]) {
			addr = search
			break
		}
	}
	if addr == Word(0) {
		return fmt.Errorf("no magic")
	}

	mem.SetWord(addr, Word(len(patch)))
	addr += 8
	if err := mem.SetMemoryRange(addr, bytes.NewReader(patch)); err != nil {
		return errors.New("failed to allocate memory for vm data")
	}
	return nil
}

// pad adds appropriate padding to buf to end at Word alignment
func pad(buf []byte) []byte {
	if len(buf)%WordSizeBytes == 0 {
		return buf
	}
	bytesToAlignment := WordSizeBytes - len(buf)%WordSizeBytes
	return append(buf, make([]byte, bytesToAlignment)...)
}
