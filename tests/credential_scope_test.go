package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hngprojects/telex_be/internal/config"
	"github.com/hngprojects/telex_be/internal/models"
	"github.com/hngprojects/telex_be/pkg/repository/storage"
	"github.com/hngprojects/telex_be/services/credentials"
	"github.com/hngprojects/telex_be/utility"
)

// TestGetCredentialByIDIsScopedToOwner covers Ziki #1188:
// GET /api/v1/credentials/:credential_id must only return a credential to the
// user and organisation that own it, and must answer 404 for everyone else so
// the route does not confirm that an id exists.
func TestGetCredentialByIDIsScopedToOwner(t *testing.T) {
	Setup()
	db := storage.Connection()

	key := []byte(config.GetConfig().Server.EncryptionKey)
	if len(key) != 32 {
		t.Skip("test config has no 32-byte Server.EncryptionKey")
	}

	encrypted, err := models.EncryptJSON(map[string]any{"api_key": "super-secret"}, key)
	assert.NoError(t, err)

	ownerID, ownerOrg := utility.GenerateUUID(), utility.GenerateUUID()
	otherID, otherOrg := utility.GenerateUUID(), utility.GenerateUUID()

	credential := models.Credential{
		ID:          utility.GenerateUUID(),
		Name:        "owner-credential",
		OrgId:       ownerOrg,
		AgentId:     utility.GenerateUUID(),
		UserId:      ownerID,
		SkillId:     utility.GenerateUUID(),
		Credentials: encrypted,
	}
	assert.NoError(t, db.Postgresql.Create(&credential).Error)
	t.Cleanup(func() {
		db.Postgresql.Exec("DELETE FROM credentials WHERE id = ?", credential.ID)
	})

	t.Run("owner in the owning organisation can read the credential", func(t *testing.T) {
		resp, code, err := credentials.GetCredentialByIDService(credential.ID, ownerID, ownerOrg, db.Postgresql)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, credential.ID, resp.ID)
		assert.Equal(t, "super-secret", resp.Credentials["api_key"])
	})

	t.Run("another user in another organisation gets 404", func(t *testing.T) {
		_, code, err := credentials.GetCredentialByIDService(credential.ID, otherID, otherOrg, db.Postgresql)
		assert.Error(t, err)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("the owner in a different organisation gets 404", func(t *testing.T) {
		_, code, _ := credentials.GetCredentialByIDService(credential.ID, ownerID, otherOrg, db.Postgresql)
		assert.Equal(t, http.StatusNotFound, code)
	})
}
