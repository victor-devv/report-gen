package store

import "database/sql"

type Store struct {
	Users        *UserStore
	RefreshToken *RefreshTokenStore
}

func New(db *sql.DB) *Store {
	return &Store{
		Users:        NewUserStore(db),
		RefreshToken: NewRefreshTokenStore(db),
	}
}
