GITCOMMIT ?= $(shell git rev-parse HEAD)
GITDATE ?= $(shell git show -s --format='%ct')
VERSION ?= v0.0.0

LDFLAGSSTRING +=-X main.GitCommit=$(GITCOMMIT)
LDFLAGSSTRING +=-X main.GitDate=$(GITDATE)

# Detect OS and architecture
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
BIN_DIR := bin
ANALYZER_BIN := $(BIN_DIR)/analyzer

# Normalize architecture naming
ifeq ($(ARCH),x86_64)
    ARCH := amd64
endif
ifeq ($(ARCH),aarch64)
    ARCH := arm64
endif

# Detect package manager
PKG_MANAGER :=
ifeq ($(OS),linux)
    ifeq ($(shell command -v apt-get),/usr/bin/apt-get)
        PKG_MANAGER := sudo apt-get install -y
    else ifeq ($(shell command -v yum),/usr/bin/yum)
        PKG_MANAGER := sudo yum install -y
    else ifeq ($(shell command -v pacman),/usr/bin/pacman)
        PKG_MANAGER := sudo pacman -Sy --noconfirm
    endif
else ifeq ($(OS),darwin)
    PKG_MANAGER := brew install
endif

# Define the binary based on OS and ARCH
ANALYZER_URL := https://github.com/ChainSafe/vm-compat/releases/latest/download/analyzer-$(OS)-$(ARCH)

# op-program-client version must ALWAYS be set to the same value (v0.0.0) to ensure exact build is reproducible
PC_LDFLAGSSTRING := $(LDFLAGSSTRING)
PC_LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/op-program/version.Version=v0.0.0
PC_LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/op-program/version.Meta=

LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/op-program/version.Version=$(VERSION)
LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/op-program/version.Meta=$(VERSION_META)

COMPAT_DIR := temp/compat

op-program: \
	op-program-host \
	op-program-client \
	op-program-client-mips

op-program-host:
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) CGO_ENABLED=0 go build -v -ldflags "$(LDFLAGSSTRING)" -o ./bin/op-program ./host/cmd/main.go

op-program-client:
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build -v -ldflags "$(PC_LDFLAGSSTRING)" -o ./bin/op-program-client ./client/cmd/main.go
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build -v -ldflags "$(PC_LDFLAGSSTRING)" -o ./bin/op-program-client-interop ./client/interopcmd/main.go

op-program-client-mips: op-program-client-mips32 op-program-client-mips64 op-program-client-mips64-interop

op-program-client-mips32:
	env GO111MODULE=on GOOS=linux GOARCH=mips GOMIPS=softfloat go build -v -ldflags "$(PC_LDFLAGSSTRING)" -o ./bin/op-program-client.elf ./client/cmd/main.go
	# verify output with: readelf -h bin/op-program-client.elf
	# result is mips32, big endian, R3000

op-program-client-mips64:
	env GO111MODULE=on GOOS=linux GOARCH=mips64 GOMIPS64=softfloat go build -v -ldflags "$(PC_LDFLAGSSTRING)" -o ./bin/op-program-client64.elf ./client/cmd/main.go
	# verify output with: readelf -h bin/op-program-client64.elf
	# result is mips64, big endian, R3000

op-program-client-mips64-interop:
	env GO111MODULE=on GOOS=linux GOARCH=mips64 GOMIPS64=softfloat go build -v -ldflags "$(PC_LDFLAGSSTRING)" -o ./bin/op-program-client-interop.elf ./client/interopcmd/main.go
	# verify output with: readelf -h bin/op-program-client-interop.elf
	# result is mips64, big endian, R3000

op-program-client-riscv:
	env GO111MODULE=on GOOS=linux GOARCH=riscv64 go build -v -gcflags="all=-d=softfloat" -ldflags "$(PC_LDFLAGSSTRING)" -o ./bin/op-program-client-riscv.elf ./client/cmd/main.go

reproducible-prestate:
	@docker build --output ./bin/ --progress plain -f Dockerfile.repro ../
	@echo "Cannon Absolute prestate hash: "
	@cat ./bin/prestate-proof.json | jq -r .pre
	@echo "Cannon64 Absolute prestate hash: "
	@cat ./bin/prestate-proof-mt64.json | jq -r .pre
	@echo "CannonInterop Absolute prestate hash: "
	@cat ./bin/prestate-proof-interop.json | jq -r .pre
.PHONY: reproducible-prestate

verify-reproducibility:
	rm -rf temp/states
	./scripts/build-prestates.sh
	env GO111MODULE=on go run ./prestates/verify/verify.go --input temp/states/versions.json
.PHONY: verify-reproducibility

clean:
	rm -rf bin "$(COMPAT_DIR)"

test:
	go test -v ./...

verify-sepolia: op-program-host op-program-client
	env GO111MODULE=on go run ./verify/sepolia/cmd/sepolia.go --l1 $$SEPOLIA_L1URL --l1.beacon $$SEPOLIA_BEACON_URL --l2 $$SEPOLIA_L2URL --datadir /tmp/test-sepolia

verify-devnet: op-program-host op-program-client
	env GO111MODULE=on go run ./verify/devnet/cmd/devnet.go --l1 http://localhost:8545 --l1.beacon http://localhost:5052 --l2 http://localhost:9545 --datadir /tmp/test-devnet

capture-mainnet-genesis: op-program-host op-program-client
	rm -rf "$(COMPAT_DIR)/mainnet-genesis" "$(COMPAT_DIR)/mainnet-genesis.tar.bz"
	env GO111MODULE=on go run ./verify/mainnet/cmd/mainnet.go --l1 $$MAINNET_L1URL --l1.beacon $$MAINNET_BEACON_URL --l2 $$MAINNET_L2URL --datadir "$(COMPAT_DIR)/mainnet-genesis" --l1.head "0x4903424f6cc2cfba7c2bf8c8f48ca46721c963fa64b411cfee3697b781e3e5f1" --l2.start "105235063" --l2.end "105235064"
	tar jcf "$(COMPAT_DIR)/mainnet-genesis.tar.bz" -C "$(COMPAT_DIR)" mainnet-genesis

capture-sepolia-delta: op-program-host op-program-client
	rm -rf "$(COMPAT_DIR)/sepolia-delta" "$(COMPAT_DIR)/sepolia-delta.tar.bz"
	env GO111MODULE=on go run ./verify/sepolia/cmd/sepolia.go --l1 $$SEPOLIA_L1URL --l1.beacon $$SEPOLIA_BEACON_URL --l2 $$SEPOLIA_L2URL --datadir "$(COMPAT_DIR)/sepolia-delta" --l1.head "0x935428728bcfcfeb2e5ba9175fd2890e52831dae221aa4d5dcffed8320edc001" --l2.start "8728200" --l2.end "8728320"
	tar jcf "$(COMPAT_DIR)/sepolia-delta.tar.bz" -C "$(COMPAT_DIR)" sepolia-delta

capture-sepolia-ecotone: op-program-host op-program-client
	rm -rf "$(COMPAT_DIR)/sepolia-ecotone" "$(COMPAT_DIR)/sepolia-ecotone.tar.bz"
	env GO111MODULE=on go run ./verify/sepolia/cmd/sepolia.go --l1 $$SEPOLIA_L1URL --l1.beacon $$SEPOLIA_BEACON_URL --l2 $$SEPOLIA_L2URL --datadir "$(COMPAT_DIR)/sepolia-ecotone" --l1.head "0x5d491a8c1e728a4e70720c09fefdaa083681a9421cd365af85220cf8bd4448a3" --l2.start "9205715" --l2.end "9205815"
	tar jcf "$(COMPAT_DIR)/sepolia-ecotone.tar.bz" -C "$(COMPAT_DIR)" sepolia-ecotone

capture-sepolia-fjord: op-program-host op-program-client
	rm -rf "$(COMPAT_DIR)/sepolia-fjord" "$(COMPAT_DIR)/sepolia-fjord.tar.bz"
	env GO111MODULE=on go run ./verify/sepolia/cmd/sepolia.go --l1 $$SEPOLIA_L1URL --l1.beacon $$SEPOLIA_BEACON_URL --l2 $$SEPOLIA_L2URL --datadir "$(COMPAT_DIR)/sepolia-fjord" --l1.head "0x93ba31bf89e54237af6e6564e69d328b2b5202adf643de4cb097431f74f4a6c1" --l2.start "15378256" --l2.end "15378356"
	tar jcf "$(COMPAT_DIR)/sepolia-fjord.tar.bz" -C "$(COMPAT_DIR)" sepolia-fjord

capture-chain-test-data: capture-mainnet-genesis capture-sepolia-delta capture-sepolia-ecotone capture-sepolia-fjord

verify-sepolia-delta: op-program-host op-program-client
	./scripts/run-compat.sh "sepolia-delta"

verify-mainnet-genesis: op-program-host op-program-client
	./scripts/run-compat.sh "mainnet-genesis"

verify-sepolia-ecotone: op-program-host op-program-client
	./scripts/run-compat.sh "sepolia-ecotone"

verify-sepolia-fjord: op-program-host op-program-client
	./scripts/run-compat.sh "sepolia-fjord"

verify-compat: verify-sepolia-delta verify-sepolia-ecotone verify-mainnet-genesis

# Install llvm-objdump if missing
.PHONY: install-llvm
install-llvm:
	@echo "Checking if llvm-objdump is installed..."
	@which llvm-objdump >/dev/null 2>&1 || (echo "Installing llvm-objdump..." && $(PKG_MANAGER) llvm)

.PHONY: fetch-analyzer
fetch-analyzer:
	@if [ -f "$(ANALYZER_BIN)" ]; then \
		echo "Analyzer binary already exists, skipping download."; \
	else \
		echo "Fetching Analyzer Binary for $(OS)-$(ARCH)..."; \
		mkdir -p $(BIN_DIR); \
		curl -L -o $(ANALYZER_BIN) $(ANALYZER_URL); \
		chmod +x $(ANALYZER_BIN); \
	fi

.PHONY: analyze-op-program-client
analyze-op-program-client: analyze-op-program-client-cannon-singlethreaded-32 analyze-op-program-client-cannon-multithreaded-64

.PHONY: analyze-op-program-client-cannon-singlethreaded-32
analyze-op-program-client-cannon-singlethreaded-32: fetch-analyzer install-llvm
	@echo "Analyzing op-program-client for cannon-singlethreaded-32..."
	$(ANALYZER_BIN) analyze  --with-trace=true --format=json  --vm-profile cannon-singlethreaded-32 --baseline-report ./compatibility-test/op-program-cannon-singlethreaded-32.json ./client/cmd/main.go

.PHONY: analyze-op-program-client-cannon-multithreaded-64
analyze-op-program-client-cannon-multithreaded-64: fetch-analyzer install-llvm
	@echo "Analyzing op-program-client for cannon-multithreaded-64..."
	$(ANALYZER_BIN) analyze  --with-trace=true --format=json  --vm-profile cannon-multithreaded-64 --baseline-report ./compatibility-test/op-program-cannon-multithreaded-64.json ./client/cmd/main.go

.PHONY: \
	op-program \
	op-program-host \
	op-program-client \
	op-program-client-mips \
	op-program-client-mips32 \
	op-program-client-mips64 \
	op-program-client-riscv \
	clean \
	test \
	capture-goerli-verify \
	verify-sepolia \
	verify-devnet \
	capture-mainnet-genesis \
	capture-sepolia-delta \
	capture-sepolia-ecotone \
	capture-chain-test-data \
	run-goerli-verify \
	run-sepolia-verify \
	run-compat
