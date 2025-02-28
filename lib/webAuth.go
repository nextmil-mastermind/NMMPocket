package lib

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"slices"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
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
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("failed to type assert credentials_list to []byte")
	}
	return json.Unmarshal(bytes, c)
}

// WebAuthnID provides the user handle of the user account. A user handle is an opaque byte sequence with a maximum
// size of 64 bytes, and is not meant to be displayed to the user.
//
// To ensure secure operation, authentication and authorization decisions MUST be made on the basis of this id
// member, not the displayName nor name members. See Section 6.1 of [RFC8266].
//
// It's recommended this value is completely random and uses the entire 64 bytes.
//
// Specification: §5.4.3. User Account Parameters for Credential Generation (https://w3c.github.io/webauthn/#dom-publickeycredentialuserentity-id)
func (user User) WebAuthnID() []byte {
	webAuthnId, err := base64.StdEncoding.DecodeString(user.WebAuthnIdB64)
	if err != nil {
		fmt.Printf("Could not base64 decode WebAuthnID from database err: %v (base64 id: %v)\n", err, user.WebAuthnIdB64)
		return []byte{}
	}
	return webAuthnId
}

// WebAuthnName provides the name attribute of the user account during registration and is a human-palatable name for the user
// account, intended only for display. For example, "Alex Müller" or "田中倫". The Relying Party SHOULD let the user
// choose this, and SHOULD NOT restrict the choice more than necessary.
//
// Specification: §5.4.3. User Account Parameters for Credential Generation (https://w3c.github.io/webauthn/#dictdef-publickeycredentialuserentity)
func (user User) WebAuthnName() string {
	return user.Username
}

// WebAuthnDisplayName provides the name attribute of the user account during registration and is a human-palatable
// name for the user account, intended only for display. For example, "Alex Müller" or "田中倫". The Relying Party
// SHOULD let the user choose this, and SHOULD NOT restrict the choice more than necessary.
//
// Specification: §5.4.3. User Account Parameters for Credential Generation (https://www.w3.org/TR/webauthn/#dom-publickeycredentialuserentity-displayname)
func (user User) WebAuthnDisplayName() string {
	if user.Name == "" {
		return user.WebAuthnName()
	}
	return user.Name
}

// WebAuthnIcon is a deprecated option.
// Deprecated: this has been removed from the specification recommendation. Suggest a blank string.
func (u User) WebAuthnIcon() string {
	return ""
}

// WebAuthnCredentials provides the list of Credential objects owned by the user.
func (user User) WebAuthnCredentials() []webauthn.Credential {
	var credentials []webauthn.Credential
	if user.WebAuthnCredentialsJSON == nil || *user.WebAuthnCredentialsJSON == "" {
		return credentials
	}
	err := json.Unmarshal([]byte(*user.WebAuthnCredentialsJSON), &credentials)
	if err != nil {
		fmt.Printf("error while unmarshalling credentials from db: %v\n", err)
		return []webauthn.Credential{}
	}
	return credentials
}

func FindUser(app *pocketbase.PocketBase, email string, collection string) (*User, error) {
	user := User{}
	err := app.DB().
		NewQuery(fmt.Sprintf(
			"SELECT id, email, username, %s, %s,%s FROM %s WHERE email={:email}",
			"webauthn_id_b64", "webauthn_credentials", "credentials_list", collection)).
		Bind(dbx.Params{"email": email}).
		One(&user)
	if err != nil {
		return nil, err
	}

	err = user.ensureWebAuthnId(app, collection)
	return &user, err
}

func (user *User) ensureWebAuthnId(app *pocketbase.PocketBase, collection string) error {
	authRecord, err := app.FindFirstRecordByData(collection, "email", user.Username)
	if err != nil {
		return apis.NewNotFoundError("User not found.", err)
	}

	// create webauthn id only if it doesnt exist yet
	if authRecord.GetString(WEBAUTHN_ID_B64_FIELDNAME) != "" {
		return nil
	}

	// create 64 bytes of random data
	randomBuffer := make([]byte, 64)
	rand.Read(randomBuffer)
	user.WebAuthnIdB64 = base64.StdEncoding.EncodeToString(randomBuffer)

	// store in database
	authRecord.Set(WEBAUTHN_ID_B64_FIELDNAME, user.WebAuthnIdB64)
	err = app.Save(authRecord)
	if err != nil {
		return err
	}

	return nil
}
func (user User) SendAuthTokenResponse(collection string, app *pocketbase.PocketBase, e *core.RequestEvent) error {
	// create auth token
	authRecord, err := app.FindFirstRecordByData(collection, "email", user.Username)
	if err != nil {
		return apis.NewNotFoundError("User not found.", err)
	}

	return apis.RecordAuthResponse(e, authRecord, "passkey", nil)
}

func (user *User) AddWebAuthnCredential(app *pocketbase.PocketBase, collection string, newCredential webauthn.Credential, device_name string) error {
	var credentials []webauthn.Credential

	if user.WebAuthnCredentialsJSON != nil && *user.WebAuthnCredentialsJSON != "" {
		err := json.Unmarshal([]byte(*user.WebAuthnCredentialsJSON), &credentials)
		if err != nil {
			return fmt.Errorf("failed to unmarshal existing credentials: %w", err)
		}
	}

	// Append the new credential
	credentials = append(credentials, newCredential)

	//Add to credentials list
	if user.CredentialsListPB == nil {
		emptyList := CredentialPBList{}
		user.CredentialsListPB = &emptyList
	}
	updatedList := append(*user.CredentialsListPB, CredentialPB{DeviceName: device_name, DeviceID: IDString(newCredential)})
	user.CredentialsListPB = &updatedList

	credentialsListJson, err := json.Marshal(user.CredentialsListPB)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials list: %w", err)
	}
	credentialsListStr := string(credentialsListJson)

	// Marshal back to JSON
	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	credentialsStr := string(credentialsJSON)
	user.WebAuthnCredentialsJSON = &credentialsStr

	// Update the record in the database
	authRecord, err := app.FindFirstRecordByData(collection, "email", user.Username)
	if err != nil {
		return apis.NewNotFoundError("User not found.", err)
	}

	authRecord.Set(WEBAUTHN_CREDENTIALS_FIELDNAME, user.WebAuthnCredentialsJSON)
	authRecord.Set("credentials_list", credentialsListStr)
	return app.Save(authRecord)
}

func (user User) DeleteWebAuthnCredential(app *pocketbase.PocketBase, collection string, credential_id string) error {
	var credentials []webauthn.Credential

	if user.WebAuthnCredentialsJSON != nil && *user.WebAuthnCredentialsJSON != "" {
		err := json.Unmarshal([]byte(*user.WebAuthnCredentialsJSON), &credentials)
		if err != nil {
			return fmt.Errorf("failed to unmarshal existing credentials: %w", err)
		}
	} else {
		return fmt.Errorf("no credentials found")
	}

	// Remove the credential with the given ID
	for i, c := range credentials {
		if IDString(c) == credential_id {
			credentials = slices.Delete(credentials, i, i+1)
			break
		}
	}
	// Marshal back to JSON
	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	credentialsStr := string(credentialsJSON)
	user.WebAuthnCredentialsJSON = &credentialsStr
	// Update the record in the database
	authRecord, err := app.FindFirstRecordByData(collection, "email", user.Username)
	if err != nil {
		return apis.NewNotFoundError("User not found.", err)
	}

	// Remove from credentials list
	if user.CredentialsListPB == nil {
		user.CredentialsListPB = new(CredentialPBList)
	}
	updatedList := []CredentialPB{}
	for _, c := range *user.CredentialsListPB {
		if c.DeviceID != credential_id {
			updatedList = append(updatedList, c)
		}
	}
	list := CredentialPBList(updatedList)
	user.CredentialsListPB = &list
	credentialsListJson, err := json.Marshal(user.CredentialsListPB)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials list: %w", err)
	}
	credentialsListStr := string(credentialsListJson)
	authRecord.Set("credentials_list", credentialsListStr)

	authRecord.Set(WEBAUTHN_CREDENTIALS_FIELDNAME, user.WebAuthnCredentialsJSON)
	return app.Save(authRecord)
}

func IDString(c webauthn.Credential) string {
	return base64.RawURLEncoding.EncodeToString(c.ID)
}
