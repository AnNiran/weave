package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iov-one/weave/cmd/bnsd/app"
	"github.com/iov-one/weave/weavetest/assert"
	"github.com/iov-one/weave/x/distribution"
)

func TestCmdResetRevenue(t *testing.T) {
	recipientsPath := mustCreateFile(t, strings.NewReader(`seq:foo/bar/1,3
seq:foo/bar/2,1
seq:foo/bar/3,20`))

	var output bytes.Buffer
	args := []string{
		"-revenue", "b1ca7e78f74423ae01da3b51e676934d9105f282",
		"-recipients", recipientsPath,
	}
	if err := cmdResetRevenue(nil, &output, args); err != nil {
		t.Fatalf("cannot create a transaction: %s", err)
	}

	var tx app.Tx
	if err := tx.Unmarshal(output.Bytes()); err != nil {
		t.Fatalf("cannot unmarshal created transaction: %s", err)
	}

	txmsg, err := tx.GetMsg()
	if err != nil {
		t.Fatalf("cannot get transaction message: %s", err)
	}
	msg := txmsg.(*distribution.ResetRevenueMsg)

	assert.Equal(t, msg.RevenueID, fromHex(t, "b1ca7e78f74423ae01da3b51e676934d9105f282"))
	assert.Equal(t, len(msg.Recipients), 3)
	assert.Equal(t, msg.Recipients[0].Weight, int32(3))
	assert.Equal(t, msg.Recipients[1].Weight, int32(1))
	assert.Equal(t, msg.Recipients[2].Weight, int32(20))
}