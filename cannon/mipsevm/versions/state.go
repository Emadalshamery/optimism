package versions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/singlethreaded"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	"github.com/ethereum-optimism/optimism/op-service/serialize"
	"github.com/ethereum/go-ethereum/log"
)

var (
	ErrUnknownVersion      = errors.New("unknown version")
	ErrJsonNotSupported    = errors.New("json not supported")
	ErrUnsupportedMipsArch = errors.New("mips architecture is not supported")
)

func LoadStateFromFile(path string) (*VersionedState, error) {
	if !serialize.IsBinaryFile(path) {
		// Always use singlethreaded for JSON states
		state, err := jsonutil.LoadJSON[singlethreaded.State](path)
		if err != nil {
			return nil, err
		}
		return NewFromState(VersionSingleThreaded, state)
	}
	return serialize.LoadSerializedBinary[VersionedState](path)
}

func NewFromState(vers StateVersion, state mipsevm.FPVMState) (*VersionedState, error) {
	switch state := state.(type) {
	case *singlethreaded.State:
		if !arch.IsMips32 {
			return nil, ErrUnsupportedMipsArch
		}
		return &VersionedState{
			Version:   vers,
			FPVMState: state,
		}, nil
	case *multithreaded.State:
		if arch.IsMips32 {
			return &VersionedState{
				Version:   vers,
				FPVMState: state,
			}, nil
		} else {
			return &VersionedState{
				Version:   vers,
				FPVMState: state,
			}, nil
		}
	default:
		return nil, fmt.Errorf("%w: %T", ErrUnknownVersion, state)
	}
}

// VersionedState deserializes a FPVMState and implements VersionedState based on the version of that state.
// It does this based on the version byte read in Deserialize
type VersionedState struct {
	Version StateVersion
	mipsevm.FPVMState
}

func (s *VersionedState) CreateVM(logger log.Logger, po mipsevm.PreimageOracle, stdOut, stdErr io.Writer, meta mipsevm.Metadata) mipsevm.FPVM {
	features := FeaturesForVersion(s.Version)
	return s.FPVMState.CreateVM(logger, po, stdOut, stdErr, meta, features)
}

func FeaturesForVersion(version StateVersion) mipsevm.FeatureToggles {
	features := mipsevm.FeatureToggles{}
	// Set any required feature toggles based on the state version here.
	if version >= VersionMultiThreaded64_v4 {
		features.SupportSysEventFd2 = true
	}
	return features
}

func (s *VersionedState) Serialize(w io.Writer) error {
	bout := serialize.NewBinaryWriter(w)
	if err := bout.WriteUInt(s.Version); err != nil {
		return err
	}
	return s.FPVMState.Serialize(w)
}

func (s *VersionedState) Deserialize(in io.Reader) error {
	bin := serialize.NewBinaryReader(in)
	if err := bin.ReadUInt(&s.Version); err != nil {
		return err
	}

	if IsSupportedSingleThreaded(s.Version) {
		if !arch.IsMips32 {
			return ErrUnsupportedMipsArch
		}
		state := &singlethreaded.State{}
		if err := state.Deserialize(in); err != nil {
			return err
		}
		s.FPVMState = state
		return nil
	} else if IsSupportedMultiThreaded(s.Version) {
		if !arch.IsMips32 {
			return ErrUnsupportedMipsArch
		}
		state := &multithreaded.State{}
		if err := state.Deserialize(in); err != nil {
			return err
		}
		s.FPVMState = state
		return nil
	} else if IsSupportedMultiThreaded64(s.Version) {
		if arch.IsMips32 {
			return ErrUnsupportedMipsArch
		}
		state := &multithreaded.State{}
		if err := state.Deserialize(in); err != nil {
			return err
		}
		s.FPVMState = state
		return nil
	} else {
		return fmt.Errorf("%w: %d", ErrUnknownVersion, s.Version)
	}
}

// MarshalJSON marshals the underlying state without adding version prefix.
// JSON states are always assumed to be single threaded
func (s *VersionedState) MarshalJSON() ([]byte, error) {
	if s.Version != VersionSingleThreaded {
		return nil, fmt.Errorf("%w for type %T", ErrJsonNotSupported, s.FPVMState)
	}
	if !arch.IsMips32 {
		return nil, ErrUnsupportedMipsArch
	}
	return json.Marshal(s.FPVMState)
}
