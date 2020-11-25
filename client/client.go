package client

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lidofinance/dc4bc/fsm/types/responses"

	sipf "github.com/lidofinance/dc4bc/fsm/state_machines/signing_proposal_fsm"

	"github.com/google/uuid"
	"github.com/lidofinance/dc4bc/client/types"
	"github.com/lidofinance/dc4bc/fsm/types/requests"

	spf "github.com/lidofinance/dc4bc/fsm/state_machines/signature_proposal_fsm"

	"github.com/lidofinance/dc4bc/fsm/state_machines"

	"github.com/lidofinance/dc4bc/fsm/fsm"
	dpf "github.com/lidofinance/dc4bc/fsm/state_machines/dkg_proposal_fsm"
	"github.com/lidofinance/dc4bc/storage"
)

const (
	pollingPeriod = time.Second
)

type Client interface {
	Poll() error
	GetLogger() *logger
	GetPubKey() ed25519.PublicKey
	GetUsername() string
	SendMessage(message storage.Message) error
	ProcessMessage(message storage.Message) error
	GetOperations() (map[string]*types.Operation, error)
	StartHTTPServer(listenAddr string) error
}

type BaseClient struct {
	sync.Mutex
	Logger   *logger
	userName string
	pubKey   ed25519.PublicKey
	ctx      context.Context
	state    State
	storage  storage.Storage
	keyStore KeyStore
}

func NewClient(
	ctx context.Context,
	userName string,
	state State,
	storage storage.Storage,
	keyStore KeyStore,
) (Client, error) {
	keyPair, err := keyStore.LoadKeys(userName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to LoadKeys: %w", err)
	}

	return &BaseClient{
		ctx:      ctx,
		Logger:   newLogger(userName),
		userName: userName,
		pubKey:   keyPair.Pub,
		state:    state,
		storage:  storage,
		keyStore: keyStore,
	}, nil
}

func (c *BaseClient) GetLogger() *logger {
	return c.Logger
}

func (c *BaseClient) GetUsername() string {
	return c.userName
}

func (c *BaseClient) GetPubKey() ed25519.PublicKey {
	return c.pubKey
}

// Poll is a main client loop, which gets new messages from an append-only log and processes them
func (c *BaseClient) Poll() error {
	tk := time.NewTicker(pollingPeriod)
	for {
		select {
		case <-tk.C:
			offset, err := c.state.LoadOffset()
			if err != nil {
				panic(err)
			}

			messages, err := c.storage.GetMessages(offset)
			if err != nil {
				return fmt.Errorf("failed to GetMessages: %w", err)
			}

			for _, message := range messages {
				if message.RecipientAddr == "" || message.RecipientAddr == c.GetUsername() {
					c.Logger.Log("Handling message with offset %d, type %s", message.Offset, message.Event)
					if err := c.ProcessMessage(message); err != nil {
						c.Logger.Log("Failed to process message with offset %d: %v", message.Offset, err)
					} else {
						c.Logger.Log("Successfully processed message with offset %d, type %s",
							message.Offset, message.Event)
					}
				}
			}
		case <-c.ctx.Done():
			log.Println("Context closed, stop polling...")
			return nil
		}
	}
}

func (c *BaseClient) SendMessage(message storage.Message) error {
	if _, err := c.storage.Send(message); err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	return nil
}

// processSignature saves a broadcasted reconstructed signature to a LevelDB
func (c *BaseClient) processSignature(message storage.Message) error {
	var (
		signature types.ReconstructedSignature
		err       error
	)
	if err = json.Unmarshal(message.Data, &signature); err != nil {
		return fmt.Errorf("failed to unmarshal reconstructed signature: %w", err)
	}
	signature.Username = message.SenderAddr
	signature.DKGRoundID = message.DkgRoundID
	return c.state.SaveSignature(signature)
}

func (c *BaseClient) ProcessMessage(message storage.Message) error {
	// save broadcasted reconstructed signature
	if fsm.Event(message.Event) == types.SignatureReconstructed {
		if err := c.processSignature(message); err != nil {
			return fmt.Errorf("failed to process signature: %w", err)
		}
		if err := c.state.SaveOffset(message.Offset + 1); err != nil {
			return fmt.Errorf("failed to SaveOffset: %w", err)
		}
		return nil
	}

	// save signing data to the same storage as we save signatures
	// This allows easy to view signing data by CLI-command
	if fsm.Event(message.Event) == sipf.EventSigningStart {
		if err := c.processSignature(message); err != nil {
			return fmt.Errorf("failed to process signature: %w", err)
		}
	}
	fsmInstance, err := c.getFSMInstance(message.DkgRoundID)
	if err != nil {
		return fmt.Errorf("failed to getFSMInstance: %w", err)
	}

	// we can't verify a message at this moment, cause we don't have public keys of participantss
	if fsm.Event(message.Event) != spf.EventInitProposal {
		if err := c.verifyMessage(fsmInstance, message); err != nil {
			return fmt.Errorf("failed to verifyMessage %+v: %w", message, err)
		}
	}

	fsmReq, err := types.FSMRequestFromMessage(message)
	if err != nil {
		return fmt.Errorf("failed to get FSMRequestFromMessage: %v", err)
	}

	resp, fsmDump, err := fsmInstance.Do(fsm.Event(message.Event), fsmReq)
	if err != nil {
		return fmt.Errorf("failed to Do operation in FSM: %w", err)
	}

	c.Logger.Log("message %s done successfully from %s", message.Event, message.SenderAddr)

	// switch FSM state by hand due to implementation specifics
	if resp.State == spf.StateSignatureProposalCollected {
		fsmInstance, err = state_machines.FromDump(fsmDump)
		if err != nil {
			return fmt.Errorf("failed get state_machines from dump: %w", err)
		}
		resp, fsmDump, err = fsmInstance.Do(dpf.EventDKGInitProcess, requests.DefaultRequest{
			CreatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("failed to Do operation in FSM: %w", err)
		}
	}
	if resp.State == dpf.StateDkgMasterKeyCollected {
		fsmInstance, err = state_machines.FromDump(fsmDump)
		if err != nil {
			return fmt.Errorf("failed get state_machines from dump: %w", err)
		}
		resp, fsmDump, err = fsmInstance.Do(sipf.EventSigningInit, requests.DefaultRequest{
			CreatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("failed to Do operation in FSM: %w", err)
		}
	}

	var operation *types.Operation
	switch resp.State {
	// if the new state is waiting for RPC to airgapped machine
	case
		spf.StateAwaitParticipantsConfirmations,
		dpf.StateDkgCommitsAwaitConfirmations,
		dpf.StateDkgDealsAwaitConfirmations,
		dpf.StateDkgResponsesAwaitConfirmations,
		dpf.StateDkgMasterKeyAwaitConfirmations,
		sipf.StateSigningAwaitPartialSigns,
		sipf.StateSigningPartialSignsCollected,
		sipf.StateSigningAwaitConfirmations:
		if resp.Data != nil {

			// if we are initiator of signing, then we don't need to confirm our participation
			if data, ok := resp.Data.(responses.SigningProposalParticipantInvitationsResponse); ok {
				initiator, err := fsmInstance.SigningQuorumGetParticipant(data.InitiatorId)
				if err != nil {
					return fmt.Errorf("failed to get SigningQuorumParticipant: %w", err)
				}
				if initiator.Username == c.GetUsername() {
					break
				}
			}

			bz, err := json.Marshal(resp.Data)
			if err != nil {
				return fmt.Errorf("failed to marshal FSM response: %w", err)
			}

			operation = &types.Operation{
				ID:            uuid.New().String(),
				Type:          types.OperationType(resp.State),
				Payload:       bz,
				DKGIdentifier: message.DkgRoundID,
				CreatedAt:     time.Now(),
			}
		}
	default:
		c.Logger.Log("State %s does not require an operation", resp.State)
	}

	// switch FSM state by hand due to implementation specifics
	if resp.State == sipf.StateSigningPartialSignsCollected {
		fsmInstance, err = state_machines.FromDump(fsmDump)
		if err != nil {
			return fmt.Errorf("failed get state_machines from dump: %w", err)
		}
		resp, fsmDump, err = fsmInstance.Do(sipf.EventSigningRestart, requests.DefaultRequest{
			CreatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("failed to Do operation in FSM: %w", err)
		}
	}

	if operation != nil {
		if err := c.state.PutOperation(operation); err != nil {
			return fmt.Errorf("failed to PutOperation: %w", err)
		}
	}

	if err := c.state.SaveOffset(message.Offset + 1); err != nil {
		return fmt.Errorf("failed to SaveOffset: %w", err)
	}

	if err := c.state.SaveFSM(message.DkgRoundID, fsmDump); err != nil {
		return fmt.Errorf("failed to SaveFSM: %w", err)
	}

	return nil
}

func (c *BaseClient) GetOperations() (map[string]*types.Operation, error) {
	return c.state.GetOperations()
}

//GetSignatures returns all signatures for the given DKG round that were reconstructed on the airgapped machine and
// broadcasted by users
func (c *BaseClient) GetSignatures(dkgID string) (map[string][]types.ReconstructedSignature, error) {
	return c.state.GetSignatures(dkgID)
}

//GetSignatureByDataHash returns a list of reconstructed signatures of the signed data broadcasted by users
func (c *BaseClient) GetSignatureByID(dkgID, sigID string) ([]types.ReconstructedSignature, error) {
	return c.state.GetSignatureByID(dkgID, sigID)
}

// getOperationJSON returns a specific JSON-encoded operation
func (c *BaseClient) getOperationJSON(operationID string) ([]byte, error) {
	operation, err := c.state.GetOperationByID(operationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get operation: %w", err)
	}

	operationJSON, err := json.Marshal(operation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal operation: %w", err)
	}
	return operationJSON, nil
}

// handleProcessedOperation handles an operation which was processed by the airgapped machine
// It checks that the operation exists in an operation pool, signs the operation, sends it to an append-only log and
// deletes it from the pool.
func (c *BaseClient) handleProcessedOperation(operation types.Operation) error {
	storedOperation, err := c.state.GetOperationByID(operation.ID)
	if err != nil {
		return fmt.Errorf("failed to find matching operation: %w", err)
	}

	if err := storedOperation.Check(&operation); err != nil {
		return fmt.Errorf("processed operation does not match stored operation: %w", err)
	}

	for i, message := range operation.ResultMsgs {
		message.SenderAddr = c.GetUsername()

		sig, err := c.signMessage(message.Bytes())
		if err != nil {
			return fmt.Errorf("failed to sign a message: %w", err)
		}
		message.Signature = sig

		operation.ResultMsgs[i] = message
	}

	if _, err := c.storage.SendBatch(operation.ResultMsgs...); err != nil {
		return fmt.Errorf("failed to post messages: %w", err)
	}

	if err := c.state.DeleteOperation(operation.ID); err != nil {
		return fmt.Errorf("failed to DeleteOperation: %w", err)
	}

	return nil
}

// getFSMInstance returns a FSM for a necessary DKG round.
func (c *BaseClient) getFSMInstance(dkgRoundID string) (*state_machines.FSMInstance, error) {
	var err error
	fsmInstance, ok, err := c.state.LoadFSM(dkgRoundID)
	if err != nil {
		return nil, fmt.Errorf("failed to LoadFSM: %w", err)
	}

	if !ok {
		fsmInstance, err = state_machines.Create(dkgRoundID)
		if err != nil {
			return nil, fmt.Errorf("failed to create FSM instance: %w", err)
		}
		bz, err := fsmInstance.Dump()
		if err != nil {
			return nil, fmt.Errorf("failed to Dump FSM instance: %w", err)
		}
		if err := c.state.SaveFSM(dkgRoundID, bz); err != nil {
			return nil, fmt.Errorf("failed to SaveFSM: %w", err)
		}
	}

	return fsmInstance, nil
}

func (c *BaseClient) signMessage(message []byte) ([]byte, error) {
	keyPair, err := c.keyStore.LoadKeys(c.userName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to LoadKeys: %w", err)
	}

	return ed25519.Sign(keyPair.Priv, message), nil
}

func (c *BaseClient) verifyMessage(fsmInstance *state_machines.FSMInstance, message storage.Message) error {
	senderPubKey, err := fsmInstance.GetPubKeyByUsername(message.SenderAddr)
	if err != nil {
		return fmt.Errorf("failed to GetPubKeyByUsername: %w", err)
	}

	if !ed25519.Verify(senderPubKey, message.Bytes(), message.Signature) {
		return errors.New("signature is corrupt")
	}

	return nil
}

func (c *BaseClient) GetFSMDump(dkgID string) (*state_machines.FSMDump, error) {
	fsmInstance, err := c.getFSMInstance(dkgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get FSM instance for DKG round ID %s: %w", dkgID, err)
	}
	return fsmInstance.FSMDump(), nil
}
