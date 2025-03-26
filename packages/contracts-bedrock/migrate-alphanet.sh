#!/usr/bin/env bash

just forge-build
forge script -vvv scripts/deploy/InteropAlphanetMigration.s.sol:InteropAlphanetMigration $@
