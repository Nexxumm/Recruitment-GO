package user

import (
	"errors"
	"regexp"
)

func RegexMatch(regex string, str string) bool {
	re := regexp.MustCompile(regex)
	return re.MatchString(str)
}

func IsValidEmail(email string) bool {
	emailPattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(emailPattern)
	return re.MatchString(email)
}

func IsValidUsername(username string) error {
	if len(username) < 4 {
		return errors.New("username must be at least 4 characters long")
	}
	usernameRegex := `^[a-zA-Z0-9_]*$`
	if !RegexMatch(usernameRegex, username) {
		return errors.New("username must only contain alphanumeric characters and underscores")
	}
	return nil
}
