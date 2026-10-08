package service

import (
	"context"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/require"
)

func TestSAMLSessionDeadlineSurvivesVerifiedLogin(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	repo := &samlFlowRepository{enterpriseFlowRepo: enterpriseFlowRepo{provider: *provider}, secret: secret, replays: map[string]bool{}}
	identity.repo, identity.users, identity.access = repo, enterpriseFlowUsers{}, NewWorkspaceAccessService(samlFlowAccessRepository{})
	start, err := identity.StartEnterpriseSSO(context.Background(), 7, 9, "/workspaces/7/identity", redirect, nil)
	require.NoError(t, err)
	deadline := now.Add(3 * time.Minute)
	response := samlResponseFixture(t, provider, redirect, repo.state.RequestID, now, idpKey, func(root *etree.Element) {
		root.FindElement("./Assertion/AuthnStatement").CreateAttr("SessionNotOnOrAfter", deadline.Format(time.RFC3339))
	}, "assertion")
	result, err := identity.CompleteSAML(context.Background(), repo.state.OpaqueState(), start.BrowserCookie, response, redirect)
	require.NoError(t, err)
	require.Equal(t, deadline, result.Assurance.ValidUntil)
	identity.now = func() time.Time { return deadline }
	require.False(t, result.Assurance.Valid(identity.now()))
}
