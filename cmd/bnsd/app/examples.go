package app

import (
	"github.com/iov-one/weave/commands"
	"github.com/iov-one/weave/crypto"
	"github.com/iov-one/weave/x"
	"github.com/iov-one/weave/x/cash"
	"github.com/iov-one/weave/x/namecoin"
	"github.com/iov-one/weave/x/nft"
	"github.com/iov-one/weave/x/nft/username"
	"github.com/iov-one/weave/x/sigs"
)

// Examples generates some example structs to dump out with testgen
func Examples() []commands.Example {
	wallet := &namecoin.Wallet{
		Name: "example",
		Coins: []*x.Coin{
			&x.Coin{Whole: 50000, Ticker: "ETH"},
			&x.Coin{Whole: 150, Fractional: 567000, Ticker: "BTC"},
		},
	}

	token := &namecoin.Token{
		Name:    "My special coin",
		SigFigs: 8,
	}

	priv := crypto.GenPrivKeyEd25519()
	pub := priv.PublicKey()
	user := &sigs.UserData{
		Pubkey:   pub,
		Sequence: 17,
	}

	dst := crypto.GenPrivKeyEd25519().PublicKey().Address()
	amt := x.NewCoin(250, 0, "ETH")
	msg := &cash.SendMsg{
		Amount: &amt,
		Dest:   dst,
		Src:    pub.Address(),
		Memo:   "Test payment",
	}

	nameMsg := &namecoin.SetWalletNameMsg{
		Address: pub.Address(),
		Name:    "myname",
	}

	tokenMsg := &namecoin.NewTokenMsg{
		Ticker:  "ATM",
		Name:    "At the moment",
		SigFigs: 3,
	}

	unsigned := Tx{
		Sum: &Tx_SendMsg{msg},
	}
	tx := unsigned
	sig, err := sigs.SignTx(priv, &tx, "test-123", 17)
	if err != nil {
		panic(err)
	}
	tx.Signatures = []*sigs.StdSignature{sig}

	owner := crypto.GenPrivKeyEd25519().PublicKey().Address()
	guest := crypto.GenPrivKeyEd25519().PublicKey().Address()
	issueToken := &username.IssueTokenMsg{
		Id:    []byte("alice@example.com"),
		Owner: owner,
		Details: username.TokenDetails{
			Addresses: []username.ChainAddress{
				{[]byte("myNet"), []byte("myChainAddress")},
			},
		},
		Approvals: []nft.ActionApprovals{
			{"update", []nft.Approval{
				{guest, nft.ApprovalOptions{Count: nft.UnlimitedCount}},
			}},
		},
	}

	addAddress := &username.AddChainAddressMsg{
		Id:      []byte("alice@example.com"),
		ChainID: []byte("myNet"),
		Address: []byte("myChainAddress"),
	}

	return []commands.Example{
		{"wallet", wallet},
		{"token", token},
		{"priv_key", priv},
		{"pub_key", pub},
		{"user", user},
		{"send_msg", msg},
		{"name_msg", nameMsg},
		{"token_msg", tokenMsg},
		{"unsigned_tx", &unsigned},
		{"signed_tx", &tx},
		{"issuetoken_msg", issueToken},
		{"add_addr_msg", addAddress},
	}
}
