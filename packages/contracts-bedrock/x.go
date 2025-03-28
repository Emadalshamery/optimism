// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package main

import (
	"github.com/lmittmann/w3"
)

var IS_SCRIPT = w3.MustNewFunc("IS_SCRIPT()")

var emitDeployed = w3.MustNewFunc("emitDeployed(bytes)")

var emittedDeployOutputs = w3.MustNewFunc("emittedDeployOutputs(uint256)")

var numEmittedDeployOutputs = w3.MustNewFunc("numEmittedDeployOutputs()")

var run = w3.MustNewFunc("run((address,address,address,bool,uint256,uint256))")

var Deployed = w3.MustNewEvent("Deployed(bytes)")
