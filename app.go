package main

import (
	db "Recruitment-GO/internal/db"
	"encoding/gob"

	"github.com/gin-contrib/sessions"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/markbates/goth"
)

type App struct {
	db           *db.Queries
	sessionStore sessions.Store
}

const (
	sessionUserKey         = "db_user_id"
	sessionTempGothUserKey = "temp_goth_user"

	RoleApplicant = "applicant"
	RoleRecruiter = "recruiter"
)

func init() {
	gob.Register(goth.User{})
	gob.Register(db.User{})
	gob.Register(pgtype.UUID{})
}
