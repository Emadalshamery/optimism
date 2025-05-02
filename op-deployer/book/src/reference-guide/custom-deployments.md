# Custom Deployments

While OP Deployer was designed primarily for use with the Optimism Superchain, it also supports managing chains that
were deployed to custom Superchain deployments. This is particularly common for RaaS providers, whose customers often
request deployments with custom L1s (or L2s, in the case of L3s) or governance. This guide will walk you through the
process of managing these chains using OP Deployer.

> [!danger]
> Chains deployed in this way are not subject to Optimism Governance. They may be running customized or unaudited
> code. Use at your own risk.

## Bootstrapping

The first step to deploying a custom Superchain is to bootstrap it onto an L1. This process will:

- Deploy contract implementations that will be shared among all OP Chains deployed using this Superchain.
- Deploy Superchain-wide management contracts like `SuperchainConfig` and `SuperchainVersions`.
- Set up ownership so that you can control the Superchain.

You will use the `bootstrap` [family][bootstrap] of commands on `op-deployer` to do this.

[bootstrap]: ../user-guide/bootstrap.md

### Superchain Bootstrap

To begin, bootstrap the Superchain onto your chosen L1 with the following command:

```shell
op-deployer bootstrap superchain \
  --l1-rpc-url="<rpc url>" \
  --private-key="<contract deployer private key>" \
  --artifacts-locator="<locator>" \
  --outfile="<path to outfile>" \
  --superchain-proxy-admin-owner="<role address>" \
  --protocol-versions-owner="<role address>" \
  --guardian="<role address>"
```

This will output a JSON file containing the addresses of the relevant contract. Keep track of this file, as you will
need it in subsequent steps.

We recommend the following these best practices when bootstrapping the Superchain:

1. Use Gnosis SAFEs for ownership roles like `guardian` and `superchain-proxy-admin-owner`. The owner **must** be a
   smart contract to support future upgrades, so a SAFE is a sensible default.
2. Use a regular EOA as the deployer. It will not have any control over the Superchain once the deployment completes.
3. Use a standard contracts tag (e.g. `tag://op-contracts/v2.0.0`). This will make upgrading easier.

### Bootstrapping Implementations

Next, you will need to bootstrap implementations onto the L1. This is done with the following command:

```shell
op-deployer bootstrap implementations \
  --artifacts-locator="<locator, should be the same as the one used in bootstrap superchain>" \
  --l1-rpc-url="<rpc url>" \
  --outfile="<path to outfile>" \
  --mips-version="2" \
  --private-key="<contract deployer private key>" \
  --protocol-versions-proxy="<address output from bootstrap superchain>" \
  --superchain-config-proxy="<address output from bootstrap superchain>" \
  --upgrade-controller="<superchain-proxy-admin-owner used in bootstrap superchain>"
```

Similar to the Superchain bootstrap command, this will output a JSON file containing the addresses of the relevant
contracts. Again, keep track of this file. The deployment scripts use `CREATE2` under the hood, so it will only
deploy contracts if their constructor arguments or implementations change. This will save time and gas.

The most important address in the implementations file is the OPCM, or OP Contracts Manager. This contract is the
factory that will deploy all the OP Chains belonging to this Superchain. It is also responsible for upgrading between
different contracts versions. There is a one-to-one mapping between OPCM, and contracts version. For this reason, it
is very important to use standard contracts versions in your deployment.

## Deploying

After bootstrapping the Superchain and implementations, you can deploy your L2 chains with the `apply` command. You
will need to specify a `configType` of `standard-overrides` and set the `opcmAddress` field in your intent to the
address of the OPCM above. For example:

> [!warning]
> Make sure that you use the same `l1ContractsLocator` and `l2ContractsLocator` as the ones used in the bootstrap
> commands. Otherwise, you may run into deployment errors.

```toml
configType = "standard-overrides"
l1ChainID = 11155420
opcmAddress = "0x..."
l1ContractsLocator = "tag://..." # must match the one used in bootstrap
l2ContractsLocator = "tag://..."

[[chains]]
# Chain configs...
```

## Upgrading

When a new contracts version is released, you will first need to re-run the `bootstrap implementations` command with
the new contracts version. This will redeploy the OPCM. Then, you can use the `upgrade` [family of commands][upgrade]
to generate calldata to upgrade from the previous version.

[upgrade]: ../user-guide/upgrade.md

You will need to use the correct sub-command for the version you are upgrading from. For example, if you are
upgrading from `v2.0.0` to `v3.0.0`, you will need to use the `upgrade v3.0.0` subcommand.

To run the upgrade command, use the following:

> [!warning]
> Upgrading between non-standard contract versions is not supported.

```shell
op-deployer upgrade <version> \
  --config <path to config JSON> \
  --l1-rpc-url="<rpc url>"
```

The contents of your config JSON should look something like this:

```json
{
  "prank": "<address of the contract that owns the chain - likely the same as the superchchain owner>",
  "opcm": "<address of new OPCM>",
  "chainConfigs": [
    {
      "systemConfigProxy": "<address of the chain's system config proxy>",
      "proxyAdmin": "<address of the chain's proxy admin>",
      "absolutePrestate": "<32-byte hash of the chain's absolute prestate>"
    }
  ]
}
```

You will get output that looks something like this:

```json
{
  "to": "<maps to the prank address>",
  "data": "<calldata>",
  "value": "0x0"
}
```

At this point, you will need to build a transaction that uses the calldata and calls the `upgrade()` method on the 
OPCM. The exact method you use to do this will depend on your tooling. As an example, you can craft a transaction 
for use with Gnosis SAFE CLI using the command below:

> [!info]
> The Gnosis SAFE UI does not support the `--delegate` flag, so the CLI is required if you're using a Gnosis SAFE.

```shell
safe-cli send-custom <owner SAFE address> <l1 rpc URL> <opcm address> 0 <calldata> --private-key <signer private key> 
--delegate
```

Note that no matter which method you use to broadcast the calldata, the call to OPCM **must** come from the smart 
contract that owns the chain and the call must be via `DELEGATECALL`. If your upgrade command reverts, it is likely 
due to one of these conditions not being met.