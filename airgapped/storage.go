package airgapped

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	bls12381 "github.com/corestario/kyber/pairing/bls12381"

	client "github.com/lidofinance/dc4bc/client/types"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	pubKeyDBKey        = "public_key"
	privateKeyDBKey    = "private_key"
	saltDBKey          = "salt_key"
	baseSeedKey        = "base_seed_key"
	operationsLogDBKey = "operations_log"
)

type RoundOperationLog map[string][]client.Operation

func (am *Machine) loadBaseSeed() error {
	seed, err := am.getBaseSeed()
	if errors.Is(err, leveldb.ErrNotFound) {
		log.Println("Base seed not initialized, generating a new one...")
		seed = make([]byte, seedSize)
		_, err = rand.Read(seed)
		if err != nil {
			return fmt.Errorf("failed to rand.Read: %w", err)
		}

		if err := am.storeBaseSeed(seed); err != nil {
			return fmt.Errorf("failed to storeBaseSeed: %w", err)
		}

		log.Println("Successfully generated a new seed")
	} else if err != nil {
		return fmt.Errorf("failed to getBaseSeed: %w", err)
	}

	am.baseSeed = seed
	am.baseSuite = bls12381.NewBLS12381Suite(am.baseSeed)

	return nil
}

func (am *Machine) storeBaseSeed(seed []byte) error {
	if err := am.db.Put([]byte(baseSeedKey), seed, nil); err != nil {
		return fmt.Errorf("failed to put baseSeed: %w", err)
	}

	return nil
}

func (am *Machine) getBaseSeed() ([]byte, error) {
	seed, err := am.db.Get([]byte(baseSeedKey), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get baseSeed: %w", err)
	}

	return seed, nil
}

func (am *Machine) storeOperation(o client.Operation) error {
	roundOperationsLog, err := am.getRoundOperationLog()
	if err != nil {
		if err == leveldb.ErrNotFound {
			return err
		}
		return fmt.Errorf("failed to get operationsLogBz from db: %w", err)
	}

	operationsLog := roundOperationsLog[o.DKGIdentifier]
	operationsLog = append(operationsLog, o)
	roundOperationsLog[o.DKGIdentifier] = operationsLog

	roundOperationsLogBz, err := json.Marshal(roundOperationsLog)
	if err != nil {
		return fmt.Errorf("failed to marshal operationsLog: %w", err)
	}

	if err := am.db.Put([]byte(operationsLogDBKey), roundOperationsLogBz, nil); err != nil {
		return fmt.Errorf("failed to put updated operationsLog: %w", err)
	}

	return nil
}

func (am *Machine) getOperationsLog(dkgIdentifier string) ([]client.Operation, error) {
	roundOperationsLog, err := am.getRoundOperationLog()
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get operationsLogBz from db: %w", err)
	}

	operationsLog, ok := roundOperationsLog[dkgIdentifier]
	if !ok {
		return nil, fmt.Errorf("operation log not found for %s", dkgIdentifier)
	}

	return operationsLog, nil
}

func (am *Machine) dropRoundOperationLog(dkgIdentifier string) error {
	roundOperationsLog, err := am.getRoundOperationLog()
	if err != nil {
		if err == leveldb.ErrNotFound {
			return err
		}
		return fmt.Errorf("failed to get operationsLogBz from db: %w", err)
	}

	roundOperationsLog[dkgIdentifier] = []client.Operation{}
	roundOperationsLogBz, err := json.Marshal(roundOperationsLog)
	if err != nil {
		return fmt.Errorf("failed to marshal operationsLog: %w", err)
	}

	if err := am.db.Put([]byte(operationsLogDBKey), roundOperationsLogBz, nil); err != nil {
		return fmt.Errorf("failed to put updated operationsLog: %w", err)
	}

	return nil
}

func (am *Machine) getRoundOperationLog() (RoundOperationLog, error) {
	operationsLogBz, err := am.db.Get([]byte(operationsLogDBKey), nil)
	if err != nil {
		return nil, err
	}

	var roundOperationsLog RoundOperationLog
	if err := json.Unmarshal(operationsLogBz, &roundOperationsLog); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stored operationsLog: %w", err)
	}

	return roundOperationsLog, nil
}

// LoadKeysFromDB load DKG keys from LevelDB
func (am *Machine) LoadKeysFromDB() error {
	pubKeyBz, err := am.db.Get([]byte(pubKeyDBKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return err
		}
		return fmt.Errorf("failed to get public key from db: %w", err)
	}

	privateKeyBz, err := am.db.Get([]byte(privateKeyDBKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return err
		}
		return fmt.Errorf("failed to get private key from db: %w", err)
	}

	salt, err := am.db.Get([]byte(saltDBKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return err
		}
		return fmt.Errorf("failed to read salt from db: %w", err)
	}

	decryptedPubKey, err := decrypt(am.encryptionKey, salt, pubKeyBz)
	if err != nil {
		return err
	}

	decryptedPrivateKey, err := decrypt(am.encryptionKey, salt, privateKeyBz)
	if err != nil {
		return err
	}

	am.pubKey = am.baseSuite.Point()
	if err = am.pubKey.UnmarshalBinary(decryptedPubKey); err != nil {
		return fmt.Errorf("failed to unmarshal public key: %w", err)
	}

	am.secKey = am.baseSuite.Scalar()
	if err = am.secKey.UnmarshalBinary(decryptedPrivateKey); err != nil {
		return fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	return nil
}

// SaveKeysToDB save DKG keys to LevelDB
func (am *Machine) SaveKeysToDB() error {
	pubKeyBz, err := am.pubKey.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal pub key: %w", err)
	}
	privateKeyBz, err := am.secKey.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	encryptedPubKey, err := encrypt(am.encryptionKey, salt, pubKeyBz)
	if err != nil {
		return err
	}
	encryptedPrivateKey, err := encrypt(am.encryptionKey, salt, privateKeyBz)
	if err != nil {
		return err
	}

	tx, err := am.db.OpenTransaction()
	if err != nil {
		return fmt.Errorf("failed to open transcation for db: %w", err)
	}
	defer tx.Discard()

	if err = tx.Put([]byte(pubKeyDBKey), encryptedPubKey, nil); err != nil {
		return fmt.Errorf("failed to put pub key into db: %w", err)
	}

	if err = tx.Put([]byte(privateKeyDBKey), encryptedPrivateKey, nil); err != nil {
		return fmt.Errorf("failed to put private key into db: %w", err)
	}

	if err = tx.Put([]byte(saltDBKey), salt, nil); err != nil {
		return fmt.Errorf("failed to put salt into db: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx for saving keys into db: %w", err)
	}

	return nil
}
