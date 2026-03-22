package domain

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RolePharmacist Role = "pharmacist"
	RoleAdmin      Role = "admin"
	RoleManager    Role = "manager"
)

func (r Role) IsValid() bool {
	switch r {
	case RolePharmacist, RoleAdmin, RoleManager:
		return true
	}
	return false
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

var (
	ErrEmptyUsername = errors.New("username cannot be empty")
	ErrEmptyPassword = errors.New("password cannot be empty")
	ErrInvalidRole   = errors.New("invalid role")
)

func NewUser(username, password string, role Role) (*User, error) {
	if username == "" {
		return nil, ErrEmptyUsername
	}
	if password == "" {
		return nil, ErrEmptyPassword
	}
	if !role.IsValid() {
		return nil, ErrInvalidRole
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now(),
	}, nil
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}
