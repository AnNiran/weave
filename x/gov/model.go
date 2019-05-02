package gov

import (
	"regexp"
	"time"

	"github.com/iov-one/weave"
	"github.com/iov-one/weave/errors"
	"github.com/iov-one/weave/orm"
)

var validTitle = regexp.MustCompile(`^[a-zA-Z0-9 _.-]{4,128}$`).MatchString

const maxElectors = 2000

func (m Electorate) Validate() error {
	switch n := len(m.Electors); {
	case n == 0:
		return errors.Wrap(errors.ErrInvalidInput, "electors must not be empty")
	case n > maxElectors:
		return errors.Wrapf(errors.ErrInvalidInput, "electors must not exceed: %d", maxElectors)
	case !validTitle(m.Title):
		return errors.Wrapf(errors.ErrInvalidInput, "title: %q", m.Title)
	}

	var totalWeight uint64
	index := make(map[string]struct{}) // check for duplicate votes
	for _, v := range m.Electors {
		if err := v.Validate(); err != nil {
			return err
		}
		totalWeight += uint64(v.Weight)
		addrKey := v.Address.String()
		if _, exists := index[addrKey]; exists {
			return errors.Wrap(errors.ErrInvalidInput, "duplicate elector entry")
		}
		index[addrKey] = struct{}{}
	}
	if m.TotalWeightElectorate != totalWeight {
		return errors.Wrap(errors.ErrInvalidInput, "total weight does not match sum")
	}
	if err := m.Admin.Validate(); err != nil {
		return errors.Wrap(err, "admin")
	}
	return nil
}

func (m Electorate) Copy() orm.CloneableData {
	p := make([]Elector, 0, len(m.Electors))
	copy(p, m.Electors)
	return &Electorate{
		Title:    m.Title,
		Electors: p,
	}
}

// Weight return the weight for the given address is in the electors list and an ok flag which
// is true when the address exists in the electors list only.
func (m Electorate) Elector(a weave.Address) (*Elector, bool) {
	for _, v := range m.Electors {
		if v.Address.Equals(a) {
			return &v, true
		}
	}
	return nil, false
}

const maxWeight = 1<<16 - 1

func (m Elector) Validate() error {
	switch {
	case m.Weight > maxWeight:
		return errors.Wrap(errors.ErrInvalidInput, "must not be greater max weight")
	case m.Weight == 0:
		return errors.Wrap(errors.ErrInvalidInput, "weight must not be empty")
	}
	return m.Address.Validate()
}

const (
	minVotingPeriodHours = 1
	maxVotingPeriodHours = 4 * 7 * 24 // 4 weeks
)

func (m ElectionRule) Validate() error {
	switch {
	case !validTitle(m.Title):
		return errors.Wrapf(errors.ErrInvalidInput, "title: %q", m.Title)
	case m.VotingPeriodHours < minVotingPeriodHours:
		return errors.Wrapf(errors.ErrInvalidInput, "min hours: %d", minVotingPeriodHours)
	case m.VotingPeriodHours > maxVotingPeriodHours:
		return errors.Wrapf(errors.ErrInvalidInput, "max hours: %d", maxVotingPeriodHours)
	}
	if err := m.Admin.Validate(); err != nil {
		return errors.Wrap(err, "admin")
	}
	return m.Threshold.Validate()
}

func (m ElectionRule) Copy() orm.CloneableData {
	return &ElectionRule{
		Title:             m.Title,
		VotingPeriodHours: m.VotingPeriodHours,
		Threshold:         m.Threshold,
	}
}

func (m Fraction) Validate() error {
	switch {
	case m.Numerator == 0:
		return errors.Wrap(errors.ErrInvalidInput, "numerator must not be 0")
	case m.Denominator == 0:
		return errors.Wrap(errors.ErrInvalidInput, "denominator must not be 0")
	case uint64(m.Numerator)*2 < uint64(m.Denominator):
		return errors.Wrap(errors.ErrInvalidInput, "must not be lower 0.5")
	case m.Numerator > m.Denominator:
		return errors.Wrap(errors.ErrInvalidInput, "must not be greater 1")
	}
	return nil
}

const (
	minDescriptionLength    = 3
	maxDescriptionLength    = 5000
	maxFutureStartTimeHours = 7 * 24 * time.Hour // 1 week
)

func (m *TextProposal) Validate() error {
	switch {
	case m.Result == TextProposal_Empty:
		return errors.Wrap(errors.ErrInvalidInput, "invalid result value")
	case m.Status == TextProposal_Invalid:
		return errors.Wrap(errors.ErrInvalidInput, "invalid status")
	case m.VotingStartTime >= m.VotingEndTime:
		return errors.Wrap(errors.ErrInvalidInput, "start time must be before end time")
	case m.VotingStartTime <= m.SubmissionTime:
		return errors.Wrap(errors.ErrInvalidInput, "start time must be after submission time")
	case len(m.Author) == 0:
		return errors.Wrap(errors.ErrInvalidInput, "author required")
	case len(m.Description) < minDescriptionLength:
		return errors.Wrapf(errors.ErrInvalidInput, "description length lower than minimum of: %d", minDescriptionLength)
	case len(m.Description) > maxDescriptionLength:
		return errors.Wrapf(errors.ErrInvalidInput, "description length exceeds: %d", maxDescriptionLength)
	case len(m.ElectorateID) == 0:
		return errors.Wrap(errors.ErrInvalidInput, "empty electorate id")
	case len(m.ElectionRuleID) == 0:
		return errors.Wrap(errors.ErrInvalidInput, "empty election rules id")
	case !validTitle(m.Title):
		return errors.Wrapf(errors.ErrInvalidInput, "title: %q", m.Title)
	}
	return nil
}

func (m TextProposal) Copy() orm.CloneableData {
	electionRuleID := make([]byte, 0, len(m.ElectionRuleID))
	copy(electionRuleID, m.ElectionRuleID)
	electorateID := make([]byte, 0, len(m.ElectorateID))
	copy(electorateID, m.ElectorateID)
	return &TextProposal{
		Title:           m.Title,
		Description:     m.Description,
		ElectionRuleID:  electionRuleID,
		ElectorateID:    electorateID,
		VotingStartTime: m.VotingStartTime,
		VotingEndTime:   m.VotingEndTime,
		SubmissionTime:  m.SubmissionTime,
		Author:          m.Author,
		VoteState:       m.VoteState,
		Status:          m.Status,
		Result:          m.Result,
	}
}

// CountVote updates the intermediate tally result by adding the new vote weight.
func (m *TextProposal) CountVote(vote Vote) error {
	oldTotal := m.VoteState.TotalVotes()
	switch vote.Voted {
	case VoteOption_Yes:
		m.VoteState.TotalYes += vote.Elector.Weight
	case VoteOption_No:
		m.VoteState.TotalNo += vote.Elector.Weight
	case VoteOption_Abstain:
		m.VoteState.TotalAbstain += vote.Elector.Weight
	default:
		return errors.Wrapf(errors.ErrInvalidInput, "%q", m.String())
	}
	if m.VoteState.TotalVotes() <= oldTotal {
		return errors.Wrap(errors.ErrHuman, "sanity overflow check failed")
	}
	return nil
}

// UndoCountVote updates the intermediate tally result by subtracting the given vote weight.
func (m *TextProposal) UndoCountVote(vote Vote) error {
	oldTotal := m.VoteState.TotalVotes()
	switch vote.Voted {
	case VoteOption_Yes:
		m.VoteState.TotalYes -= vote.Elector.Weight
	case VoteOption_No:
		m.VoteState.TotalNo -= vote.Elector.Weight
	case VoteOption_Abstain:
		m.VoteState.TotalAbstain -= vote.Elector.Weight
	default:
		return errors.Wrapf(errors.ErrInvalidInput, "%q", m.String())
	}
	if m.VoteState.TotalVotes() >= oldTotal {
		return errors.Wrap(errors.ErrHuman, "sanity overflow check failed")
	}
	return nil
}

// Tally calls the final calculation on the votes and sets the status of the proposal according to the
// election rules threshold.
func (m *TextProposal) Tally() error {
	if m.Result != TextProposal_Undefined {
		return errors.Wrapf(errors.ErrInvalidState, "result exists: %q", m.Result.String())
	}
	if m.Status != TextProposal_Submitted {
		return errors.Wrapf(errors.ErrInvalidState, "unexpected status: %q", m.Status.String())
	}
	if m.VoteState.Accepted() {
		m.Result = TextProposal_Accepted
	} else {
		m.Result = TextProposal_Rejected
	}
	m.Status = TextProposal_Closed
	return nil
}

// Accepted returns the result of the `(yes*denominator) > (numerator*total_electors_weight)` calculation.
func (m TallyResult) Accepted() bool {
	return uint64(m.TotalYes)*uint64(m.Threshold.Denominator) > m.TotalWeightElectorate*uint64(m.Threshold.Numerator)
}

// TotalVotes returns the sum of yes, no, abstain votes.
func (m TallyResult) TotalVotes() uint64 {
	return uint64(m.TotalYes) + uint64(m.TotalNo) + uint64(m.TotalAbstain)
}

// validate vote object contains valid elector and voted option
func (m Vote) Validate() error {
	if err := m.Elector.Validate(); err != nil {
		return errors.Wrap(err, "invalid elector")
	}
	if m.Voted == VoteOption_Invalid {
		return errors.Wrap(errors.ErrInvalidInput, "invalid vote option")
	}
	return nil
}

func (m Vote) Copy() orm.CloneableData {
	return &Vote{
		Elector: m.Elector,
		Voted:   m.Voted,
	}
}