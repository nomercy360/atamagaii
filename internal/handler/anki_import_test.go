package handler

import (
	"atamagaii/internal/testutils"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHandleAnkiImport(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	require.NotEmpty(t, resp.Token, "Expected non-empty JWT token")
}
