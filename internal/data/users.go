package data

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Sub       string    `json:"sub"`
	CreatedAt time.Time `json:"-"`
}

type UserModel struct {
	DB *sql.DB
}

func (m UserModel) Insert(user *User) error {
	query := `
		insert into users (email, username, oidc_sub)
		values ($1, $2, $3)
		returning id, email, username, oidc_sub, created_at`

	args := []any{user.Email, user.Username, user.Sub}

	return m.DB.QueryRow(query, args...).Scan(&user.ID, &user.Email, &user.Username, &user.Sub, &user.CreatedAt)
}

func (m UserModel) GetBySub(sub string) (*User, error) {
	query := `select id, email, username, oidc_sub, created_at from users where oidc_sub = $1`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, sub).Scan(&user.ID, &user.Email, &user.Username, &user.Sub, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &user, nil
}
