package ceremony

import (
	mapset "github.com/deckarep/golang-set"
	"github.com/idena-network/idena-go/blockchain"
	"github.com/idena-network/idena-go/blockchain/types"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/config"
	"github.com/idena-network/idena-go/core/state"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/rlp"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidationCeremony_getFlipsToSolve(t *testing.T) {
	require := require.New(t)

	myKey := common.Address{0x1, 0x2, 0x3}

	flipsCids := [][]byte{{0x1}, {0x2}, {0x3}, {0x4}, {0x5}}

	fliptsPerCandidate := [][]int{{0, 1, 2}, {4, 2, 1}, {1, 2, 3}, {1, 2, 4}, {0, 1, 3}}

	result := getFlipsToSolve(myKey, getParticipants(myKey, 0, 5), fliptsPerCandidate, flipsCids)
	shouldBe := [][]byte{{0x1}, {0x2}, {0x3}}
	require.Equal(shouldBe, result)

	result = getFlipsToSolve(myKey, getParticipants(myKey, 3, 5), fliptsPerCandidate, flipsCids)
	shouldBe = [][]byte{{0x2}, {0x3}, {0x5}}
	require.Equal(shouldBe, result)

	result = getFlipsToSolve(myKey, getParticipants(myKey, 4, 5), fliptsPerCandidate, flipsCids)
	shouldBe = [][]byte{{0x1}, {0x2}, {0x4}}
	require.Equal(shouldBe, result)
}

func TestValidationCeremony_getFlipsToSolve_fewFlips(t *testing.T) {
	require := require.New(t)

	myKey := common.Address{0x1, 0x2, 0x3}

	flipsCids := [][]byte{{0x1}, {0x2}, {0x3}, {0x4}, {0x5}}

	fliptsPerCandidate := [][]int{{0, 1, 6}, {4, 2, 8}, {1, 2, 4}, {1, 2, 3}, {6, 7, 8}}

	result := getFlipsToSolve(myKey, getParticipants(myKey, 0, 5), fliptsPerCandidate, flipsCids)
	shouldBe := [][]byte{{0x1}, {0x2}, {0x2}}
	require.Equal(shouldBe, result)

	result = getFlipsToSolve(myKey, getParticipants(myKey, 4, 5), fliptsPerCandidate, flipsCids)
	shouldBe = [][]byte{{0x2}, {0x3}, {0x4}}
	require.Equal(shouldBe, result)
}

func getParticipants(myKey common.Address, myIndex int, length int) []*candidate {
	participants := make([]*candidate, 0)

	for i := 0; i < length; i++ {
		if i == myIndex {
			participants = append(participants, &candidate{
				Address: myKey,
			})
		} else {
			participants = append(participants, &candidate{
				Address: common.Address{byte(i)},
			})
		}
	}
	return participants
}

func Test_determineNewIdentityState(t *testing.T) {

	type data struct {
		prev                state.IdentityState
		shortScore          float32
		longScore           float32
		totalScore          float32
		totalQualifiedFlips uint32
		missed              bool
		expected            state.IdentityState
		noQualShort         bool
		noQualLong          bool
	}

	cases := []data{
		{
			state.Killed,
			0, 0, 0, 0, true,
			state.Killed,false,false,
		},
		{
			state.Invite,
			1, 1, 1, 110, false,
			state.Killed,false,false,
		},
		{
			state.Candidate,
			MinShortScore, MinLongScore, MinTotalScore, 11, false,
			state.Newbie,false,false,
		},
		{
			state.Candidate,
			MinShortScore, MinLongScore, MinTotalScore, 11, true,
			state.Killed,false,false,
		},
		{
			state.Newbie,
			MinShortScore, MinLongScore, MinTotalScore, 11, false,
			state.Verified,false,false,
		},
		{
			state.Newbie,
			MinShortScore, MinLongScore, MinTotalScore, 10, false,
			state.Newbie,false,false,
		},
		{
			state.Newbie,
			MinShortScore, MinLongScore, MinTotalScore, 11, true,
			state.Killed,false,false,
		},
		{
			state.Newbie,
			0.4, 0.8, 1, 11, false,
			state.Killed,false,false,
		},
		{
			state.Newbie,
			MinShortScore, MinLongScore, MinTotalScore, 8, false,
			state.Newbie,false,false,
		},
		{
			state.Verified,
			MinShortScore, MinLongScore, MinTotalScore, 10, false,
			state.Killed,false,false,
		},
		{
			state.Verified,
			0, 0, 0, 0, true,
			state.Suspended,false,false,
		},
		{
			state.Verified,
			0, 0, 0, 0, false,
			state.Killed,false,false,
		},
		{
			state.Suspended,
			MinShortScore, MinLongScore, MinTotalScore, 10, false,
			state.Verified,false,false,
		},
		{
			state.Suspended,
			1, 0.8, 0, 10, true,
			state.Zombie,false,false,
		},
		{
			state.Zombie,
			MinShortScore, 0, MinTotalScore, 10, false,
			state.Verified,false,false,
		},
		{
			state.Zombie,
			1, 0, 0, 10, true,
			state.Killed,false,false,
		},
		{
			state.Candidate,
			MinShortScore, 0, 0, 5, false,
			state.Candidate,true,false,
		},
		{
			state.Candidate,
			MinShortScore-0.1, 0, 0, 5, false,
			state.Killed,false,true,
		},
		{
			state.Newbie,
			MinShortScore, 0, 0.1, 5, false,
			state.Newbie,true,false,
		},
		{
			state.Newbie,
			MinShortScore, 0, 0.1, 5, false,
			state.Newbie,false,true,
		},
		{
			state.Newbie,
			MinShortScore, 0, 0.1, 11, false,
			state.Killed,false,true,
		},
		{
			state.Newbie,
			MinShortScore-0.1, 0, 0.1, 9, false,
			state.Killed,false,true,
		},
		{
			state.Verified,
			MinShortScore-0.1, 0, 0.1, 10, false,
			state.Verified,true,false,
		},
		{
			state.Verified,
			MinShortScore-0.1, 0, 1.1, 10, false,
			state.Killed,false,true,
		},
		{
			state.Suspended,
			MinShortScore-0.1, 0, 0.1, 10, false,
			state.Suspended,true,false,
		},
		{
			state.Suspended,
			MinShortScore-0.1, 0, 1.1, 10, false,
			state.Killed,false,true,
		},
		{
			state.Zombie,
			MinShortScore-0.1, 0, 0.1, 10, false,
			state.Zombie,true,false,
		},
		{
			state.Zombie,
			MinShortScore, 0, 0.1, 10, false,
			state.Killed,false,true,
		},
	}

	require := require.New(t)

	for _, c := range cases {
		require.Equal(c.expected, determineNewIdentityState(state.Identity{State: c.prev}, c.shortScore, c.longScore, c.totalScore, c.totalQualifiedFlips, c.missed, c.noQualShort, c.noQualLong))
	}
}

func Test_getNotApprovedFlips(t *testing.T) {
	// given
	vc := ValidationCeremony{}
	_, app, _, _ := blockchain.NewTestBlockchain(false, make(map[common.Address]config.GenesisAllocation))
	var candidates []*candidate
	var flipsPerAuthor map[int][][]byte
	var flips [][]byte
	for i := 0; i < 3; i++ {
		key, _ := crypto.GenerateKey()
		c := candidate{
			Address: crypto.PubkeyToAddress(key.PublicKey),
		}
		candidates = append(candidates, &c)
	}
	for i := 0; i < 5; i++ {
		flips = append(flips, []byte{byte(i)})
	}
	flipsPerAuthor = make(map[int][][]byte)
	flipsPerAuthor[0] = [][]byte{
		flips[0],
		flips[1],
		flips[2],
	}
	flipsPerAuthor[1] = [][]byte{
		flips[3],
	}
	flipsPerAuthor[2] = [][]byte{
		flips[4],
	}
	addr := candidates[0].Address
	app.State.SetRequiredFlips(addr, 3)
	approvedAddr := candidates[1].Address
	app.State.SetRequiredFlips(approvedAddr, 3)

	vc.candidates = candidates
	vc.flips = flips
	vc.flipsPerAuthor = flipsPerAuthor
	vc.appState = app

	approvedCandidates := mapset.NewSet()
	approvedCandidates.Add(approvedAddr)

	// when
	result := vc.getNotApprovedFlips(approvedCandidates)

	// then
	r := require.New(t)
	r.Equal(3, result.Cardinality())
	r.True(result.Contains(0))
	r.True(result.Contains(1))
	r.True(result.Contains(2))
}

func Test_flipPos(t *testing.T) {
	flips := [][]byte{
		{1, 2, 3},
		{1, 2, 3, 4},
		{2, 3, 4},
	}
	r := require.New(t)
	r.Equal(-1, flipPos(flips, []byte{1, 2, 3, 4, 5}))
	r.Equal(0, flipPos(flips, []byte{1, 2, 3}))
	r.Equal(1, flipPos(flips, []byte{1, 2, 3, 4}))
	r.Equal(2, flipPos(flips, []byte{2, 3, 4}))
}

func Test_analizeAuthors(t *testing.T) {
	vc := ValidationCeremony{}

	auth1 := common.Address{1}
	auth2 := common.Address{2}
	auth3 := common.Address{3}
	auth4 := common.Address{4}
	auth5 := common.Address{5}

	vc.flips = [][]byte{{0x0}, {0x1}, {0x2}, {0x3}, {0x4}, {0x5}, {0x6}, {0x7}, {0x8}, {0x9}}
	vc.flipAuthorMap = map[common.Hash]common.Address{
		rlp.Hash([]byte{0x0}): auth1,
		rlp.Hash([]byte{0x1}): auth1,
		rlp.Hash([]byte{0x2}): auth1,

		rlp.Hash([]byte{0x3}): auth2,
		rlp.Hash([]byte{0x4}): auth2,

		rlp.Hash([]byte{0x5}): auth3,
		rlp.Hash([]byte{0x6}): auth3,

		rlp.Hash([]byte{0x7}): auth4,
		rlp.Hash([]byte{0x8}): auth4,

		rlp.Hash([]byte{0x9}): auth5,
	}

	qualification := []FlipQualification{
		{status: Qualified},
		{status: WeaklyQualified},
		{status: NotQualified},

		{status: Qualified, answer: types.Inappropriate},
		{status: Qualified},

		{status: WeaklyQualified, wrongWords: true},
		{status: Qualified},

		{status: NotQualified},
		{status: NotQualified},

		{status: QualifiedByNone},
	}

	bad, good := vc.analizeAuthors(qualification)

	require.Contains(t, bad, auth2)
	require.Contains(t, bad, auth3)
	require.Contains(t, bad, auth4)
	require.Contains(t, bad, auth5)
	require.NotContains(t, bad, auth1)

	require.Contains(t, good, auth1)
	require.Equal(t, 1, good[auth1].WeakFlips)
	require.Equal(t, 1, good[auth1].StrongFlips)
}

func Test_incSuccessfulInvites(t *testing.T) {

	god := common.Address{0x1}
	auth1 := common.Address{0x2}
	badAuth := common.Address{0x3}

	authors := &types.ValidationAuthors{
		BadAuthors: map[common.Address]struct{}{badAuth: {}},
		GoodAuthors: map[common.Address]*types.ValidationResult{
			auth1: {StrongFlips: 1, WeakFlips: 1, SuccessfulInvites: 0},
		},
	}

	incSuccessfulInvites(authors, god, state.Identity{
		State: state.Verified,
		Inviter: &state.TxAddr{
			Address: god,
		},
	}, state.Newbie)

	incSuccessfulInvites(authors, god, state.Identity{
		State: state.Candidate,
		Inviter: &state.TxAddr{
			Address: auth1,
		},
	}, state.Newbie)

	incSuccessfulInvites(authors, god, state.Identity{
		State: state.Candidate,
		Inviter: &state.TxAddr{
			Address: badAuth,
		},
	}, state.Newbie)

	incSuccessfulInvites(authors, god, state.Identity{
		State: state.Candidate,
		Inviter: &state.TxAddr{
			Address: god,
		},
	}, state.Newbie)

	require.Equal(t, authors.GoodAuthors[auth1].SuccessfulInvites, 1)
	require.Equal(t, authors.GoodAuthors[god].SuccessfulInvites, 1)
	require.NotContains(t, authors.GoodAuthors, badAuth)
}

func Test_determineIdentityBirthday(t *testing.T) {
	identity := state.Identity{}
	identity.Birthday = 1
	identity.State = state.Newbie
	require.Equal(t, uint16(1), determineIdentityBirthday(2, identity, state.Newbie))
}
