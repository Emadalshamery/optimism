# Style Guide

This style guide outlines common patterns and anti-patterns used by op-testlib. Following this guide not only improves
consistency, it helps keep the separation of requirements (in test files) from implementation details (in testlib
implementation), which in turn ensures tests are maintainable even as the number of tests keeps increasing over time.

## Entry Points

What are the key entry points for the system? Nodes/services, users, contracts??

## Action Methods

Methods that perform actions will typically have three steps:

1. Check (and if needed, wait) for any required preconditions
2. Perform the action, allowing components to fully process the effects of it
3. Assert that the action completed. These are intended to be a sanity check to ensure tests fail fast if something
   doesn't work as expected. Options may be provided to perform more detailed or specific assertions

## Verification Methods

Verification methods in op-testlib provide additional assertions about the state of the system, beyond the minimal
assertions performed by action methods.

Verification methods should include any required waiting or retrying.

Verification methods should generally only be used in tests to assert the specific behaviours the test is covering.
Avoid adding additional verification steps in a test to assert that setup actions were performed correctly - such
assertions should be built into the action methods. While sanity checking setup can be useful, adding additional
verification method calls into tests makes it harder to see what the test is actually intending to cover and increases
the number of places that need to be updated if the behaviour being verified changes in the future.

### Avoid Getter Methods

op-testlib generally avoids exposing methods that return data from the system state. Instead verification methods are
exposed which combine the fetching and assertion of the data. This allows the testlib to handle any waiting or retrying
that may be necessary (or become necessary). This avoids a common source of flakiness where tests assume an asynchronous
operation will have completed instead of explicitly waiting for the expected final state.


```go
// Avoid: the balance of an account is data from the system which changes over time
block := node.GetBalance(user)

// Good: use a verification method
node.VerifyBalance(user, 10 * constants.Eth)

// Better? Select the entry point to be as declarative as possible
user.VerifyBalacne(10 * constants.Eth) // implementation could verify balance on all nodes automatically
```


Note however that this doesn't mean that testlib methods never return anything. While returning raw data is avoided,
returning objects that represent something in the system is ok. e.g.

```go
claim := game.RootClaim()

// Waits for op-challenger to counter the root claim and returns a value representing that counter claim
// which can expose further verification or action methods.
counter := claim.VerifyCountered()
counter.VerifyClaimant(honestChallenger)
counter.Attack()
```

## Method Arguments

Required inputs to methods are specified as normal parameters, so type checking enforces their presence.

Optional inputs to methods are specified by a config struct and accept a vararg of functions that can update that struct.
This is roughly inline with the typical opts pattern in Golang but with significantly reduced boilerplate code since
so many methods wil define their own config. With* methods are only provided for the most common optional args and
tests will normally supply a custom function that sets all the optional values they need at once.

### Common Optional Arguments

Common options can be extracted to a reusable struct (e.g. ChainOpts above) which may expose helper methods to aid
test readability and reduce boilerplate. For example if many methods accept a list of chains, a `ChainOpts` struct
could be used to reduce boilerplate code:

```go
type ChainOpts struct {
	Chains []*Chain
}

func (c *ChainOpts) SetChains(chains ...*Chain) {
	c.Chains = chains
}

func (c *ChainOpts) AddChain(chain *Chain) {
	c.Chains = append(c.Chains, chain)
}
```
