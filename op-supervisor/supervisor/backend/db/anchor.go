package db

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// ForceInitialized marks the chain database as initialized, even if it is not.
// This function is for testing purposes only and should not be used in production code.
func (db *ChainsDB) ForceInitialized(id eth.ChainID) {
	db.initialized.Set(id, struct{}{})
}

func (db *ChainsDB) IsInitialized(id eth.ChainID) bool {
	_, ok := db.initialized.Get(id)
	return ok
}

func (db *ChainsDB) InitializeWithAnchor(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	db.initFromAnchor(id, anchor)
}

// UpdatePreActivationHead updates the in-memory tracked head for a chain pre-activation
func (db *ChainsDB) UpdatePreActivationHead(id eth.ChainID, block eth.BlockRef) {
	// Only update if in pre-activation mode
	inPreActivation, ok := db.preActivationMode.Get(id)
	if !ok || !inPreActivation {
		db.logger.Debug("not updating head for chain not in pre-activation mode", "chain", id)
		return
	}

	// Update the tracked head
	db.logger.Info("updating pre-activation head", "chain", id, "block", block)
	db.preActivationHeads.Set(id, block)

	// Mark as initialized for API consistency with normal mode
	if !db.IsInitialized(id) {
		db.initialized.Set(id, struct{}{})
	}
}

// GetPreActivationHead returns the currently tracked head for a chain in pre-activation mode
func (db *ChainsDB) GetPreActivationHead(id eth.ChainID) (eth.BlockRef, bool) {
	// Check if we're in pre-activation mode
	inPreActivation, ok := db.preActivationMode.Get(id)
	if !ok || !inPreActivation {
		return eth.BlockRef{}, false
	}

	// Return the tracked head if available
	head, ok := db.preActivationHeads.Get(id)
	return head, ok
}

// IsInPreActivationMode checks if a chain is currently in pre-activation mode
func (db *ChainsDB) IsInPreActivationMode(id eth.ChainID) bool {
	inPreActivation, ok := db.preActivationMode.Get(id)
	return ok && inPreActivation
}

// ExitPreActivationMode transitions a chain from pre-activation to normal mode
// This should be called when interop activation is detected
func (db *ChainsDB) ExitPreActivationMode(id eth.ChainID, anchor types.DerivedBlockRefPair) error {
	// Check if we're in pre-activation mode
	inPreActivation, ok := db.preActivationMode.Get(id)
	if !ok || !inPreActivation {
		return fmt.Errorf("chain %s is not in pre-activation mode", id)
	}

	db.logger.Info("exiting pre-activation mode", "chain", id, "anchor", anchor)

	// Clear pre-activation state
	db.preActivationMode.Delete(id)
	db.preActivationHeads.Delete(id)

	// Reset initialization state
	db.initialized.Delete(id)

	// Initialize using the anchor point
	db.initFromAnchor(id, anchor)

	return nil
}

// InitializePreActivation sets up pre-activation tracking for a chain
func (db *ChainsDB) InitializePreActivation(id eth.ChainID, block eth.BlockRef) {
	// Check if we're already in pre-activation mode instead of just initialized
	inPreActivation, ok := db.preActivationMode.Get(id)
	if ok && inPreActivation {
		db.logger.Debug("chain already in pre-activation mode")
		return
	}

	db.logger.Info("setting up pre-activation tracking", "chain", id, "block", block)

	// Set pre-activation mode
	db.preActivationMode.Set(id, true)

	// Set initial head
	db.preActivationHeads.Set(id, block)

	// Mark as initialized for API consistency
	db.initialized.Set(id, struct{}{})
}

func (db *ChainsDB) initFromAnchor(id eth.ChainID, anchor types.DerivedBlockRefPair) {
	// Check if the chain database is already initialized
	if db.IsInitialized(id) {
		db.logger.Debug("chain database already initialized")
		return
	}
	db.logger.Debug("initializing chain database from anchor point")

	// Initialize the local and cross safe databases
	if err := db.maybeInitSafeDB(id, anchor); err != nil {
		db.logger.Warn("failed to initialize local and cross safe databases", "err", err)
		return
	}

	// Initialize the events database
	if err := db.maybeInitEventsDB(id, anchor); err != nil {
		db.logger.Warn("failed to initialize events database", "err", err)
		return
	}

	// Mark the chain database as initialized
	db.initialized.Set(id, struct{}{})
	db.anchorBlocks.Set(id, anchor)
}

// maybeInitSafeDB initializes the chain database if it is not already initialized
// it checks if the Local Safe database is empty, and loads both the Local and Cross Safe databases
// with the anchor point if they are empty.
func (db *ChainsDB) maybeInitSafeDB(id eth.ChainID, anchor types.DerivedBlockRefPair) error {
	logger := db.logger.New("chain", id, "derived", anchor.Derived, "source", anchor.Source)
	localDB, ok := db.localDBs.Get(id)
	if !ok {
		return types.ErrUnknownChain
	}
	first, err := localDB.First()
	if errors.Is(err, types.ErrFuture) {
		logger.Info("local database is empty, initializing")
		if err := db.initializedUpdateCrossSafe(id, anchor.Source, anchor.Derived); err != nil {
			return err
		}
		// "anchor" is not a node, so failure to update won't be caught by any SyncNode
		db.initializedUpdateLocalSafe(id, anchor.Source, anchor.Derived, "anchor")
	} else if err != nil {
		return fmt.Errorf("failed to check if chain database is initialized: %w", err)
	} else {
		logger.Debug("chain database already initialized")
		if first.Derived.Hash != anchor.Derived.Hash ||
			first.Source.Hash != anchor.Source.Hash {
			return fmt.Errorf("local database (%s) does not match anchor point (%s): %w",
				first,
				anchor,
				types.ErrConflict)
		}
	}
	return nil
}

func (db *ChainsDB) maybeInitEventsDB(id eth.ChainID, anchor types.DerivedBlockRefPair) error {
	logger := db.logger.New("chain", id, "derived", anchor.Derived, "source", anchor.Source)
	seal, _, _, err := db.OpenBlock(id, 0)
	if errors.Is(err, types.ErrFuture) {
		logger.Debug("initializing events database")
		err := db.initializedSealBlock(id, anchor.Derived)
		if err != nil {
			return err
		}
		logger.Info("Initialized events database")
	} else if err != nil {
		return fmt.Errorf("failed to check if logDB is initialized: %w", err)
	} else {
		logger.Debug("Events database already initialized")
		if seal.Hash != anchor.Derived.Hash {
			return fmt.Errorf("events database (%s) does not match anchor point (%s): %w",
				seal,
				anchor,
				types.ErrConflict)
		}
	}
	return nil
}