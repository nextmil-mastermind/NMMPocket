package passkeys

import (
	"encoding/json"
	"fmt"
)

type User struct {
	Id                      string            `db:"id" json:"id"`
	Username                string            `db:"email" json:"username"`
	Name                    string            `db:"username" json:"name"`
	WebAuthnIdB64           string            `db:"webauthn_id_b64" json:"webauthn_id_b64"`
	WebAuthnCredentialsJSON *string           `db:"webauthn_credentials" json:"webauthn_credentials"`
	CredentialsListPB       *CredentialPBList `db:"credentials_list" json:"credentials_list"`
}

const (
	WEBAUTHN_CREDENTIALS_FIELDNAME string = "webauthn_credentials"
	WEBAUTHN_ID_B64_FIELDNAME      string = "webauthn_id_b64"
)

type CredentialPB struct {
	DeviceName string `json:"device_name"`
	DeviceID   string `json:"device_id"`
}

// Define a new type
type CredentialPBList []CredentialPB

// Implement the sql.Scanner interface
func (c *CredentialPBList) Scan(src any) error {
	if src == nil {
		*c = nil
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("unsupported type %T", src)
	}
	return json.Unmarshal(data, c)
}
