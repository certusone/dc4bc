package dkg_proposal_fsm

import (
	"github.com/depools/dc4bc/fsm/fsm"
	"github.com/depools/dc4bc/fsm/state_machines/internal"
	"sync"
)

const (
	FsmName = "dkg_proposal_fsm"

	StateDkgInitial = StateDkgPubKeysAwaitConfirmations

	StateDkgPubKeysAwaitConfirmations = fsm.State("state_dkg_pub_keys_await_confirmations")
	// Canceled
	StateDkgPubKeysAwaitCanceled          = fsm.State("state_dkg_pub_keys_await_canceled")
	StateDkgPubKeysAwaitCanceledByTimeout = fsm.State("state_dkg_pub_keys_await_canceled_by_timeout")
	// Confirmed
	// StateDkgPubKeysAwaitConfirmed = fsm.State("state_dkg_pub_keys_await_confirmed")

	// Sending dkg commits
	StateDkgCommitsAwaitConfirmations = fsm.State("state_dkg_commits_await_confirmations")
	// Canceled
	StateDkgCommitsAwaitCanceled          = fsm.State("state_dkg_commits_await_canceled")
	StateDkgCommitsAwaitCanceledByTimeout = fsm.State("state_dkg_commits_await_canceled_by_timeout")
	// Confirmed
	StateDkgCommitsAwaitConfirmed = fsm.State("state_dkg_commits_await_confirmed")

	// Sending dkg deals
	StateDkgDealsAwaitConfirmations = fsm.State("state_dkg_deals_await_confirmations")
	// Canceled
	StateDkgDealsAwaitCanceled          = fsm.State("state_dkg_deals_await_canceled")
	StateDkgDealsAwaitCanceledByTimeout = fsm.State("state_dkg_deals_await_canceled_by_timeout")
	// Confirmed
	//StateDkgDealsAwaitConfirmed = fsm.State("state_dkg_deals_await_confirmed")

	StateDkgResponsesAwaitConfirmations = fsm.State("state_dkg_responses_await_confirmations")
	// Canceled
	StateDkgResponsesAwaitCanceled          = fsm.State("state_dkg_responses_await_canceled")
	StateDkgResponsesAwaitCanceledByTimeout = fsm.State("state_dkg_responses_sending_canceled_by_timeout")
	// Confirmed
	StateDkgResponsesAwaitConfirmed = fsm.State("state_dkg_responses_await_confirmed")

	StateDkgMasterKeyAwaitConfirmations     = fsm.State("state_dkg_master_key_await_confirmations")
	StateDkgMasterKeyAwaitCanceled          = fsm.State("state_dkg_master_key_await_canceled")
	StateDkgMasterKeyAwaitCanceledByTimeout = fsm.State("state_dkg_master_key_await_canceled_by_timeout")

	// Events

	eventAutoDKGInitialInternal = fsm.Event("event_dkg_init_internal")

	EventDKGPubKeyConfirmationReceived = fsm.Event("event_dkg_pub_key_confirm_received")
	EventDKGPubKeyConfirmationError    = fsm.Event("event_dkg_pub_key_confirm_canceled_by_error")

	eventAutoDKGValidatePubKeysConfirmationInternal = fsm.Event("event_dkg_pub_keys_validate_internal")

	eventDKGSetPubKeysConfirmationCanceledByTimeoutInternal = fsm.Event("event_dkg_pub_keys_confirm_canceled_by_timeout_internal")
	eventDKGSetPubKeysConfirmationCanceledByErrorInternal   = fsm.Event("event_dkg_pub_keys_confirm_canceled_by_error_internal")
	eventDKGSetPubKeysConfirmedInternal                     = fsm.Event("event_dkg_pub_keys_confirmed_internal")

	EventDKGCommitConfirmationReceived                 = fsm.Event("event_dkg_commit_confirm_received")
	EventDKGCommitConfirmationError                    = fsm.Event("event_dkg_commit_confirm_canceled_by_error")
	eventDKGCommitsConfirmationCancelByTimeoutInternal = fsm.Event("event_dkg_commits_confirm_canceled_by_timeout_internal")
	eventDKGCommitsConfirmationCancelByErrorInternal   = fsm.Event("event_dkg_commits_confirm_canceled_by_error_internal")
	eventDKGCommitsConfirmedInternal                   = fsm.Event("event_dkg_commits_confirmed_internal")
	eventAutoDKGValidateConfirmationCommitsInternal    = fsm.Event("event_dkg_commits_validate_internal")

	EventDKGDealConfirmationReceived                 = fsm.Event("event_dkg_deal_confirm_received")
	EventDKGDealConfirmationError                    = fsm.Event("event_dkg_deal_confirm_canceled_by_error")
	eventDKGDealsConfirmationCancelByTimeoutInternal = fsm.Event("event_dkg_deals_confirm_canceled_by_timeout_internal")
	eventDKGDealsConfirmationCancelByErrorInternal   = fsm.Event("event_dkg_deals_confirm_canceled_by_error_internal")
	eventDKGDealsConfirmedInternal                   = fsm.Event("event_dkg_deals_confirmed_internal")
	eventAutoDKGValidateConfirmationDealsInternal    = fsm.Event("event_dkg_deals_validate_internal")

	EventDKGResponseConfirmationReceived                = fsm.Event("event_dkg_response_confirm_received")
	EventDKGResponseConfirmationError                   = fsm.Event("event_dkg_response_confirm_canceled_by_error")
	eventDKGResponseConfirmationCancelByTimeoutInternal = fsm.Event("event_dkg_response_confirm_canceled_by_timeout_internal")
	eventDKGResponseConfirmationCancelByErrorInternal   = fsm.Event("event_dkg_response_confirm_canceled_by_error_internal")
	eventDKGResponsesConfirmedInternal                  = fsm.Event("event_dkg_responses_confirmed_internal")
	eventAutoDKGValidateResponsesConfirmationInternal   = fsm.Event("event_dkg_responses_validate_internal")

	EventDKGMasterKeyConfirmationReceived                = fsm.Event("event_dkg_master_key_confirm_received")
	EventDKGMasterKeyConfirmationError                   = fsm.Event("event_dkg_master_key_confirm_canceled_by_error")
	eventDKGMasterKeyConfirmationCancelByTimeoutInternal = fsm.Event("event_dkg_master_key_confirm_canceled_by_timeout_internal")
	eventDKGMasterKeyConfirmationCancelByErrorInternal   = fsm.Event("event_dkg_master_key_confirm_canceled_by_error_internal")
	eventDKGMasterKeyConfirmedInternal                   = fsm.Event("event_dkg_master_key_confirmed_internal")
	eventAutoDKGValidateMasterKeyConfirmationInternal    = fsm.Event("event_dkg_master_key_validate_internal")

	EventDKGMasterKeyRequiredInternal = fsm.Event("event_dkg_master_key_required_internal")
)

type DKGProposalFSM struct {
	*fsm.FSM
	payload   *internal.DumpedMachineStatePayload
	payloadMu sync.RWMutex
}

func New() internal.DumpedMachineProvider {
	machine := &DKGProposalFSM{}

	machine.FSM = fsm.MustNewFSM(
		FsmName,
		StateDkgInitial,
		[]fsm.EventDesc{

			// Init
			// Switch to pub keys required
			// 	{Name: eventDKGPubKeysSendingRequiredAuto, SrcState: []fsm.State{StateDkgInitial}, DstState: StateDkgPubKeysAwaitConfirmations, IsInternal: true, IsAuto: true, AutoRunMode: fsm.EventRunAfter},

			// {Name: eventAutoDKGInitialInternal, SrcState: []fsm.State{StateDkgPubKeysAwaitConfirmations}, DstState: StateDkgPubKeysAwaitConfirmations, IsInternal: true, IsAuto: true, AutoRunMode: fsm.EventRunBefore},

			// Pub keys sending
			{Name: EventDKGPubKeyConfirmationReceived, SrcState: []fsm.State{StateDkgPubKeysAwaitConfirmations}, DstState: StateDkgPubKeysAwaitConfirmations},
			// Canceled
			{Name: EventDKGPubKeyConfirmationError, SrcState: []fsm.State{StateDkgPubKeysAwaitConfirmations}, DstState: StateDkgPubKeysAwaitCanceled},

			{Name: eventAutoDKGValidatePubKeysConfirmationInternal, SrcState: []fsm.State{StateDkgPubKeysAwaitConfirmations}, DstState: StateDkgPubKeysAwaitConfirmations, IsInternal: true, IsAuto: true},

			{Name: eventDKGSetPubKeysConfirmationCanceledByTimeoutInternal, SrcState: []fsm.State{StateDkgPubKeysAwaitConfirmations}, DstState: StateDkgPubKeysAwaitCanceledByTimeout, IsInternal: true},
			// Confirmed
			{Name: eventDKGSetPubKeysConfirmedInternal, SrcState: []fsm.State{StateDkgPubKeysAwaitConfirmations}, DstState: StateDkgCommitsAwaitConfirmations, IsInternal: true},

			// Switch to commits required

			// Commits
			{Name: EventDKGCommitConfirmationReceived, SrcState: []fsm.State{StateDkgCommitsAwaitConfirmations}, DstState: StateDkgCommitsAwaitConfirmations},
			// Canceled
			{Name: EventDKGCommitConfirmationError, SrcState: []fsm.State{StateDkgCommitsAwaitConfirmations}, DstState: StateDkgCommitsAwaitCanceled},
			{Name: eventDKGCommitsConfirmationCancelByTimeoutInternal, SrcState: []fsm.State{StateDkgCommitsAwaitConfirmations}, DstState: StateDkgCommitsAwaitCanceledByTimeout, IsInternal: true},

			{Name: eventAutoDKGValidateConfirmationCommitsInternal, SrcState: []fsm.State{StateDkgCommitsAwaitConfirmations}, DstState: StateDkgCommitsAwaitConfirmations, IsInternal: true, IsAuto: true},

			// Confirmed
			{Name: eventDKGCommitsConfirmedInternal, SrcState: []fsm.State{StateDkgCommitsAwaitConfirmations}, DstState: StateDkgDealsAwaitConfirmations, IsInternal: true},

			// Deals
			{Name: EventDKGDealConfirmationReceived, SrcState: []fsm.State{StateDkgDealsAwaitConfirmations}, DstState: StateDkgDealsAwaitConfirmations},
			// Canceled
			{Name: EventDKGDealConfirmationError, SrcState: []fsm.State{StateDkgDealsAwaitConfirmations}, DstState: StateDkgDealsAwaitCanceled},
			{Name: eventDKGDealsConfirmationCancelByTimeoutInternal, SrcState: []fsm.State{StateDkgDealsAwaitConfirmations}, DstState: StateDkgDealsAwaitConfirmations, IsInternal: true},
			{Name: eventAutoDKGValidateConfirmationDealsInternal, SrcState: []fsm.State{StateDkgDealsAwaitConfirmations}, DstState: StateDkgDealsAwaitConfirmations, IsInternal: true, IsAuto: true},

			{Name: eventDKGDealsConfirmedInternal, SrcState: []fsm.State{StateDkgDealsAwaitConfirmations}, DstState: StateDkgResponsesAwaitConfirmations, IsInternal: true},

			// Responses
			{Name: EventDKGResponseConfirmationReceived, SrcState: []fsm.State{StateDkgResponsesAwaitConfirmations}, DstState: StateDkgResponsesAwaitConfirmations},
			// Canceled
			{Name: EventDKGResponseConfirmationError, SrcState: []fsm.State{StateDkgResponsesAwaitConfirmations}, DstState: StateDkgResponsesAwaitCanceled},
			{Name: eventDKGResponseConfirmationCancelByTimeoutInternal, SrcState: []fsm.State{StateDkgResponsesAwaitConfirmations}, DstState: StateDkgResponsesAwaitCanceledByTimeout, IsInternal: true},

			{Name: eventAutoDKGValidateResponsesConfirmationInternal, SrcState: []fsm.State{StateDkgResponsesAwaitConfirmations}, DstState: StateDkgResponsesAwaitConfirmations, IsInternal: true, IsAuto: true},

			{Name: eventDKGResponsesConfirmedInternal, SrcState: []fsm.State{StateDkgResponsesAwaitConfirmations}, DstState: StateDkgMasterKeyAwaitConfirmations, IsInternal: true},

			// Master key

			{Name: EventDKGMasterKeyConfirmationReceived, SrcState: []fsm.State{StateDkgMasterKeyAwaitConfirmations}, DstState: StateDkgMasterKeyAwaitConfirmations},
			{Name: EventDKGMasterKeyConfirmationError, SrcState: []fsm.State{StateDkgMasterKeyAwaitConfirmations}, DstState: StateDkgMasterKeyAwaitCanceled},
			{Name: eventDKGMasterKeyConfirmationCancelByTimeoutInternal, SrcState: []fsm.State{StateDkgMasterKeyAwaitConfirmations}, DstState: StateDkgMasterKeyAwaitCanceledByTimeout, IsInternal: true},

			{Name: eventAutoDKGValidateMasterKeyConfirmationInternal, SrcState: []fsm.State{StateDkgMasterKeyAwaitConfirmations}, DstState: StateDkgMasterKeyAwaitConfirmations, IsInternal: true, IsAuto: true},

			{Name: eventDKGMasterKeyConfirmedInternal, SrcState: []fsm.State{StateDkgMasterKeyAwaitConfirmations}, DstState: fsm.StateGlobalDone, IsInternal: true},

			// Done
			// {Name: EventDKGMasterKeyRequiredInternal, SrcState: []fsm.State{StateDkgResponsesAwaitConfirmations}, DstState: fsm.StateGlobalDone, IsInternal: true},
		},
		fsm.Callbacks{

			EventDKGPubKeyConfirmationReceived:              machine.actionPubKeyConfirmationReceived,
			EventDKGPubKeyConfirmationError:                 machine.actionConfirmationError,
			eventAutoDKGValidatePubKeysConfirmationInternal: machine.actionValidateDkgProposalPubKeys,

			EventDKGCommitConfirmationReceived:              machine.actionCommitConfirmationReceived,
			EventDKGCommitConfirmationError:                 machine.actionConfirmationError,
			eventAutoDKGValidateConfirmationCommitsInternal: machine.actionValidateDkgProposalAwaitCommits,

			EventDKGDealConfirmationReceived:              machine.actionDealConfirmationReceived,
			EventDKGDealConfirmationError:                 machine.actionConfirmationError,
			eventAutoDKGValidateConfirmationDealsInternal: machine.actionValidateDkgProposalAwaitDeals,

			EventDKGResponseConfirmationReceived:              machine.actionResponseConfirmationReceived,
			EventDKGResponseConfirmationError:                 machine.actionConfirmationError,
			eventAutoDKGValidateResponsesConfirmationInternal: machine.actionValidateDkgProposalAwaitResponses,

			EventDKGMasterKeyConfirmationReceived:             machine.actionMasterKeyConfirmationReceived,
			EventDKGMasterKeyConfirmationError:                machine.actionConfirmationError,
			eventAutoDKGValidateMasterKeyConfirmationInternal: machine.actionValidateDkgProposalAwaitMasterKey,
		},
	)
	return machine
}

func (m *DKGProposalFSM) SetUpPayload(payload *internal.DumpedMachineStatePayload) {
	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	m.payload = payload
}
