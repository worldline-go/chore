package claims

import (
	"encoding/json"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/worldline-go/auth/claims"
)

var (
	KeyUserID    = "sub"
	KeyTokenType = "chore_token_type"
	KeyTokenID   = "chore_token_id"
	KeyRoles     = "roles"
)

type Custom struct {
	claims.Custom

	TokenType string `json:"chore_token_type,omitempty"`
	TokenID   string `json:"chore_token_id,omitempty"`
}

func (c *Custom) UnmarshalJSON(b []byte) error {
	type newCustom Custom
	err := json.Unmarshal(b, (*newCustom)(c))
	if err != nil {
		return err
	}

	c.TokenID, _ = c.Map[KeyTokenID].(string)
	c.TokenType, _ = c.Map[KeyTokenType].(string)

	return nil
}

func NewClaims() jwt.Claims {
	return &Custom{
		Custom: claims.Custom{},
	}
}

func NewMapClaims(tokenID, userID uuid.UUID, tokenType string, groups []string) map[string]interface{} {
	// modify groups to add chore_ prefix
	addDefaultGroup := true
	roles := make([]string, 0, len(groups))
	for _, g := range groups {
		roles = append(roles, "chore_"+g)

		if g == "user" {
			addDefaultGroup = false
		}
	}

	if addDefaultGroup {
		roles = append(roles, "chore_user")
	}

	return map[string]interface{}{
		KeyTokenID:   tokenID,
		KeyUserID:    userID,
		KeyTokenType: tokenType,
		"roles":      roles,
	}
}
