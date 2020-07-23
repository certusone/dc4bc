package dkg

import (
	"errors"
	"fmt"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	vss "go.dedis.ch/kyber/v3/share/vss/pedersen"
)

type DKG struct {
	instance      *dkg.DistKeyGenerator
	deals         map[string]*dkg.Deal
	commits       map[string]kyber.Point
	pubkeys       PKStore
	pubKey        kyber.Point
	secKey        kyber.Scalar
	suite         *bn256.Suite
	participantID int
}

func Init() *DKG {
	var (
		d DKG
	)

	d.suite = bn256.NewSuiteG2()
	d.secKey = d.suite.Scalar().Pick(d.suite.RandomStream())
	d.pubKey = d.suite.Point().Mul(d.secKey, nil)

	d.deals = make(map[string]*dkg.Deal)
	d.commits = make(map[string]kyber.Point)

	return &d
}

func (d *DKG) GetPubKey() kyber.Point {
	return d.pubKey
}

func (d *DKG) StorePubKey(participant string, pk kyber.Point) bool {
	return d.pubkeys.Add(&PK2Participant{
		Participant: participant,
		PK:          pk,
	})
}

func (d *DKG) InitDKGInstance(t int) (err error) {
	d.instance, err = dkg.NewDistKeyGenerator(d.suite, d.secKey, d.pubkeys.GetPKs(), t)
	if err != nil {
		return err
	}
	return nil
}

func (d *DKG) GetCommits() []kyber.Point {
	return d.instance.GetDealer().Commits()
}

func (d *DKG) StoreCommit(participant string, commit kyber.Point) {
	d.commits[participant] = commit
}

func (d *DKG) GetDeals() (map[int]*dkg.Deal, error) {
	deals, err := d.instance.Deals()
	if err != nil {
		return nil, err
	}
	for _, deal := range deals {
		d.participantID = int(deal.Index) // Same for each deal.
		break
	}
	return deals, nil
}

func (d *DKG) StoreDeal(participant string, deal *dkg.Deal) {
	d.deals[participant] = deal
}

func (d *DKG) processDeals() error {
	for _, deal := range d.deals {
		if deal.Index == uint32(d.participantID) {
			continue
		}
		resp, err := d.instance.ProcessDeal(deal)
		if err != nil {
			return err
		}

		// Commits verification.
		allVerifiers := d.instance.Verifiers()
		verifier := allVerifiers[deal.Index]
		commitsOK, err := d.processDealCommits(verifier, deal)
		if err != nil {
			return err
		}

		// If something goes wrong, party complains.
		if !resp.Response.Status || !commitsOK {
			return fmt.Errorf("failed to process deals")
		}
	}
	return nil
}

func (d *DKG) processDealCommits(verifier *vss.Verifier, deal *dkg.Deal) (bool, error) {
	decryptedDeal, err := verifier.DecryptDeal(deal.Deal)
	if err != nil {
		return false, err
	}

	commitsData, ok := d.commits.indexToData[int(deal.Index)]
	if !ok {
		return false, err
	}
	var originalCommits []kyber.Point
	for _, commitData := range commitsData {
		commit, ok := commitData.(kyber.Point)
		if !ok {
			return false, fmt.Errorf("failed to cast commit data to commit type")
		}
		originalCommits = append(originalCommits, commit)
	}

	if len(originalCommits) != len(decryptedDeal.Commitments) {
		return false, errors.New("number of original commitments and number of commitments in the deal are not met")
	}

	for i := range originalCommits {
		if !originalCommits[i].Equal(decryptedDeal.Commitments[i]) {
			return false, errors.New("commits are different")
		}
	}

	return true, nil
}

func (d *DKG) Reconstruct() error {
	if d.instance == nil || !d.instance.Certified() {
		return fmt.Errorf("dkg instance is not ready")
	}

	distKeyShare, err := d.instance.DistKeyShare()
	if err != nil {
		return fmt.Errorf("failed to get DistKeyShare: %v", err)
	}

	masterPubKey := share.NewPubPoly(d.suite, nil, distKeyShare.Commitments())
	//.....
}