package tendermint

import (
	"time"

	lite "github.com/tendermint/tendermint/lite2"
	tmtypes "github.com/tendermint/tendermint/types"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clientexported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
)

// CheckValidityAndUpdateState checks if the provided header is valid and updates
// the consensus state if appropriate. It returns an error if:
// - the client or header provided are not parseable to tendermint types
// - the header is invalid
// - header height is lower than the latest client height
// - light client header verification fails
//
// Tendermint client validity checking uses the bisection algorithm described
// in the [Tendermint spec](https://github.com/tendermint/spec/blob/master/spec/consensus/light-client.md).
func CheckValidityAndUpdateState(
	clientState clientexported.ClientState,
	oldHeader, newHeader clientexported.Header,
	oldHeaderNextVals *tmtypes.ValidatorSet,
	chainID string,
	trustingPeriod time.Duration,
) (clientexported.ClientState, clientexported.ConsensusState, error) {
	tmClientState, ok := clientState.(ClientState)
	if !ok {
		return nil, nil, sdkerrors.Wrap(
			clienttypes.ErrInvalidClientType, "light client is not from Tendermint",
		)
	}

	tmHeader1, ok := oldHeader.(Header)
	if !ok {
		return nil, nil, sdkerrors.Wrap(
			clienttypes.ErrInvalidHeader, "header is not from Tendermint",
		)
	}

	tmHeader2, ok := newHeader.(Header)
	if !ok {
		return nil, nil, sdkerrors.Wrap(
			clienttypes.ErrInvalidHeader, "header is not from Tendermint",
		)
	}

	if err := checkValidity(
		tmClientState, tmHeader1, tmHeader2, oldHeaderNextVals, chainID, trustingPeriod); err != nil {
		return nil, nil, err
	}

	tmClientState, consensusState := update(tmClientState, tmHeader2)
	return tmClientState, consensusState, nil
}

// checkValidity checks if the Tendermint header is valid
func checkValidity(
	clientState ClientState,
	oldHeader,
	newHeader Header,
	oldHeaderNextVals *tmtypes.ValidatorSet,
	chainID string,
	trustingPeriod time.Duration,
) error {
	if newHeader.Height <= oldHeader.Height {
		return sdkerrors.Wrapf(
			clienttypes.ErrInvalidHeader,
			"new header height ≤ old header height (%d <= %d)", newHeader.Height, oldHeader.Height,
		)
	}

	if newHeader.GetHeight() < clientState.LatestHeight {
		return sdkerrors.Wrapf(
			clienttypes.ErrInvalidHeader,
			"header height < latest client state height (%d < %d)", newHeader.Height, clientState.LatestHeight,
		)
	}

	// basic consistency check for the new header
	if err := newHeader.ValidateBasic(chainID); err != nil {
		return err
	}

	// call tendermint light client verification function
	return lite.Verify(
		chainID, &oldHeader.SignedHeader, oldHeaderNextVals, &newHeader.SignedHeader,
		newHeader.ValidatorSet, trustingPeriod, time.Now(), lite.DefaultTrustLevel,
	)
}

// update the consensus state from a new header
func update(clientState ClientState, header Header) (ClientState, ConsensusState) {
	clientState.LatestHeight = header.GetHeight()
	consensusState := ConsensusState{
		Root:             commitment.NewRoot(header.AppHash),
		ValidatorSetHash: header.ValidatorSet.Hash(),
	}

	return clientState, consensusState
}
