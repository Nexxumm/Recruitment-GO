package profile

import (
	db "Recruitment-GO/internal/db"
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) RegisterHandlers(router *gin.Engine) {

	router.DELETE("/profile", s.DeleteUser)
}

type returnUser struct {
	ID       pgtype.UUID `json:"id"`
	Email    string      `json:"email"`
	Name     string      `json:"name"`
	Role     string      `json:"role"`
	GoogleID string      `json:"google_id"`
}

func fromGetDB(user db.GetUserRow) *returnUser {

	var ID pgtype.UUID
	if user.ID.Valid {
		ID = user.ID
	} else {
		return nil
	}
	return &returnUser{
		ID:       ID,
		Email:    user.Email,
		Name:     user.Name,
		Role:     user.Role,
		GoogleID: user.GoogleID,
	}
}

func (s *Service) DeleteUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, ok := userID.(pgtype.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	if err := s.queries.DeleteUser(context.Background(), id); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.Status(http.StatusOK)
}
