package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type CardModel struct {
	DB *sql.DB
}

type CardSpeed int

const (
	SLOW CardSpeed = iota
	FAST
)

type Card struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Atk         int         `json:"atk"`         //-1 for spells
	Def         int         `json:"def"`         //-1 for spells
	Type        string      `json:"type"`        //Creature, Spell
	Speed       CardSpeed   `json:"-"`           //0 for sorcery speed 1 for instant speed
	IsPermanent bool        `json:"isPermanent"` //0 if a permanent 1 if not
	Quantity    int         `json:"-"`
	Tribute     int         `json:"tribute"`
	CanAttack   bool        `json:"canAttack"`
	Keywords    KeywordList `json:"keywords"`
}

type CardKeyword struct {
	Name   string `json:"name"`
	Factor int    `json:"factor"`
}

type KeywordList []CardKeyword

func (kl *KeywordList) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, kl)
}

func (c CardModel) GetDeck() ([]Card, error) {
	query := `SELECT dc.quantity, c.id, c.name, c.atk, c.def, c.type, c.speed, c.is_permanent,c.tribute,
    
    		COALESCE(
        	JSON_AGG(
            JSON_BUILD_OBJECT('name', k.name, 'factor', ck.factor)
        	) FILTER (WHERE k.name IS NOT NULL), 
        	'[]'
    		) AS keywords
			FROM deck_cards dc
			JOIN cards c ON dc.card_id = c.id
			LEFT JOIN card_keywords ck ON c.id = ck.card_id
			LEFT JOIN keywords k ON ck.keyword_id = k.id
			WHERE dc.deck_id = 1
			GROUP BY dc.quantity, c.id;
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := c.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var deck []Card

	for rows.Next() {
		var c Card
		err := rows.Scan(&c.Quantity,
			&c.ID,
			&c.Name,
			&c.Atk,
			&c.Def,
			&c.Type,
			&c.Speed,
			&c.IsPermanent,
			&c.Tribute,
			&c.Keywords)
		if err != nil {
			return nil, err
		}

		deck = append(deck, c)
	}

	var properDeck []Card
	for _, card := range deck {
		for i := 0; i < card.Quantity; i++ {
			properDeck = append(properDeck, card)
		}

	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return properDeck, nil
}
