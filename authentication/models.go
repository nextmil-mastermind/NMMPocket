package authentication

import "github.com/golang-jwt/jwt/v5"

type UserClaims struct {
	UserID string   `json:"userId"`
	Role   []string `json:"role"`
	Name   string   `json:"name"`
	jwt.RegisteredClaims
}

type StudentClaims struct {
	UserID            string   `json:"userId"`
	Role              []string `json:"role"`
	StudentExpiration string   `json:"student_expiration"`
	PayType           string   `json:"pay_type"`
	Name              string   `json:"name"`
	jwt.RegisteredClaims
}

type MemberClaims struct {
	UserID           string `json:"userId"`
	Group            string `json:"group"`
	MemberExpiration string `json:"expiration"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	Email            string `json:"email"`
	jwt.RegisteredClaims
}
