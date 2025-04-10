package authentication

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

var sharedSecret []byte

func Init() {
	sharedSecret = []byte(os.Getenv("shared_secret"))
}

func generateUserJWT(userID, name string, role []string) (string, error) {
	claims := UserClaims{
		userID,
		role,
		name,
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // 24-hour expiry
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(sharedSecret)
}
func generateMemberJWT(userID, first_name, last_name, email, expiration, group string) (string, error) {
	claims := MemberClaims{
		UserID:           userID,
		Group:            group,
		MemberExpiration: expiration,
		FirstName:        first_name,
		LastName:         last_name,
		Email:            email,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // 24-hour expiry
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(sharedSecret)
}
func Routes(router *router.Router[*core.RequestEvent]) {
	router.GET("/user_token", func(e *core.RequestEvent) error {
		//get the auth user
		authRecord := e.Auth
		if authRecord.Collection().Name != "users" {
			return apis.NewUnauthorizedError("Unauthorized.", nil)
		}
		token, err := generateUserJWT(authRecord.Id, authRecord.GetString("name"), authRecord.Get("role").([]string))
		if err != nil {
			return apis.NewInternalServerError("Failed to generate token.", err)
		}
		return e.JSON(200, map[string]string{"token": token})

	}).Bind(apis.RequireAuth())
	router.GET("/member_token", func(e *core.RequestEvent) error {
		//get the auth user
		authRecord := e.Auth
		if authRecord.Collection().Name != "members" {
			return apis.NewUnauthorizedError("Unauthorized.", nil)
		}
		token, err := generateMemberJWT(authRecord.Id, authRecord.GetString("first_name"), authRecord.GetString("last_name"), authRecord.GetString("email"), authRecord.Get("expiration").(string), authRecord.Get("group").(string))
		if err != nil {
			return apis.NewInternalServerError("Failed to generate token.", err)
		}
		return e.JSON(200, map[string]string{"token": token})

	})
}
