package data

import "database/sql"

type Models struct {
	Users UserModel
	Cards CardModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Users: UserModel{DB: db},
		Cards: CardModel{DB: db},
	}
}
