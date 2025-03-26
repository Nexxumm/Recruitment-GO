package auth

import (
	"api/user"
	"errors"
)

func ValidateUserRequest(req apiUser) error {
	if IsPasswordValid(req.Password) != nil {
		return IsPasswordValid(req.Password)
	}
	if user.IsValidUsername(req.Username) != nil {
		return user.IsValidUsername(req.Username)
	}
	if !user.IsValidEmail(req.Email) {
		return errors.New("invalid email format")
	}

	return nil
}
