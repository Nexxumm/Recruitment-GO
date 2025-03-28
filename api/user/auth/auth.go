package auth

import (
	"context"
	db "db/sqlc"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) RegisterHandlers(router *gin.Engine) {
	router.POST("/auth/register", s.CreateUser)
	router.POST("/auth/login", s.LoginUser)
	router.POST("/auth/logout", s.LogoutUser)
}

type apiUser struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Password string    `pg:"-" binding:"required,min=8,max=32"`
	Email    string    `json:"email"`
}

type loginUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type returnUser struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
}

func fromCreateDB(user db.CreateUserRow) *returnUser {
	return &returnUser{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}
}

func (s *Service) CreateUser(c *gin.Context) {
	var request apiUser
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ValidateUserRequest(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash := hashAndSalt([]byte(request.Password))

	if hash == "Error hashing password" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error hashing password"})
		return
	}

	// Create user
	params := db.CreateUserParams{
		Username: request.Username,
		Password: hash,
		Email:    request.Email,
	}
	user, err := s.queries.CreateUser(context.Background(), params)
	if err != nil {
		errMessage := AuthErrMessage(err.Error())
		if errMessage != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMessage})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "something went wrong"})
		return
	}

	// Build response
	response := fromCreateDB(user)
	c.IndentedJSON(http.StatusCreated, response)
}

func (s *Service) LoginUser(c *gin.Context) {
	var request loginUser
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := s.queries.GetUserID(context.Background(), request.Username)
	currentPassword, _ := s.queries.GetPassword(context.Background(), userID)

	if !comparePasswords(currentPassword, []byte(request.Password)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect username or password"})
		return
	}

	authSecret := os.Getenv("AUTH_SECRET")
	if authSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour * 24 * 15).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenStr, err := token.SignedString([]byte(authSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	// Set token in cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", tokenStr, 3600*24*15, "", "", false, true)

	// Build response
	c.JSON(http.StatusOK, gin.H{"message": "login successful, cookie set", "token": tokenStr})
}

func (s *Service) LogoutUser(c *gin.Context) {
	// simply removing the cookie will log the user out
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "", "", false, true)
}
