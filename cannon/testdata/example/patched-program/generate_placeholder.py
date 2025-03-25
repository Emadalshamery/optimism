#!/usr/bin/env python3

import struct
import os

FILE_SIZE = 1024

START_MAGIC = b'\x0e\x24\xfd\x47\x91\x6a\xa3\xac\x16\x26\x4a\x6c\x87\x43\x71\xfc\xdc\xc3\x48\x50\x5d\xac\x3e\xc4\xbb\x13\xdf\x04\xc8\xc6\xf2\xe5'  # keccak('optimism 4 lyfe')
END_MAGIC = 0x69696969    # 4 bytes

# Create binary data
with open("placeholder.bin", "wb") as f:
    assert(len(START_MAGIC) == 32)
    f.write(START_MAGIC)

    filler_size = FILE_SIZE - 32 - 4
    # Create random filler bytes just to inhibit any delta-encoding by the Go compiler if it exists
    filler = os.urandom(filler_size)
    f.write(filler)

    f.write(struct.pack(">I", END_MAGIC))

file_size = os.path.getsize("placeholder.bin")
print(f"Generated placeholder.bin: {file_size} bytes")

# sanity check
with open("placeholder.bin", "rb") as f:
    start_bytes = f.read(32)
    f.seek(-8, 2)
    end_bytes = f.read(8)

print(f"Start bytes: {start_bytes.hex()}")
print(f"End bytes: {end_bytes.hex()}")
