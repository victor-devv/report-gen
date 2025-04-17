package store

import "database/sql"

type Store struct {
	Users        *UserStore
	RefreshToken *RefreshTokenStore
	Reports      *ReportStore
}

func New(db *sql.DB) *Store {
	return &Store{
		Users:        NewUserStore(db),
		RefreshToken: NewRefreshTokenStore(db),
		Reports:      NewReportStore(db),
	}
}
