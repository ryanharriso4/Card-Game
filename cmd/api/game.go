package main

import (
	"encoding/json"

	"cardgame.ryanharris.net/internal/data"
)

type GameEventType string
type GamePhaseType string
type GamePlayerAction string
type GameLossReason string

const (
	EventChangePhase GameEventType    = "PHASE_CHANGED"
	EventStateChange GameEventType    = "STAT_CHANGE"
	EventCardMoved   GameEventType    = "CARD_MOVED"
	EventGameOver    GameEventType    = "GAME_OVER"
	PhaseMain        GamePhaseType    = "MAIN_PHASE"
	PhaseCombat      GamePhaseType    = "COMBAT_PHASE"
	ActionPlay       GamePlayerAction = "PLAY_CARD"
	ActionAttack     GamePlayerAction = "ATTACK"
	ActionNextPhase  GamePlayerAction = "NEXT_PHASE"
	OpponentDeckOut  GameLossReason   = "opponent decked out"
	PlayerDeckOut    GameLossReason   = "you decked out"
	PlayerDied       GameLossReason   = "you ran out of life points"
	OpponentDied     GameLossReason   = "opponent ran out of life points"
)

type ReasonLoss int

const (
	DECKEDOUT ReasonLoss = iota
	DAMAGE
	NOLOSS
)

type PlayerState struct {
	ID        string      `json:"-"`
	Health    int         `json:"health"`
	Hand      []data.Card `json:"hand"`
	Deck      []data.Card `json:"-"`
	Graveyard []data.Card `json:"graveyard"`
	Board     []data.Card `json:"board"`
	HandSize  int         `json:"hand_size"`
	DeckSize  int         `json:"deck_size"`
}

type GameState struct {
	Turn       int                     `json:"turn"`
	Players    map[string]*PlayerState `json:"players"`
	ActiveTurn string                  `json:"-"`
	Phase      string                  `json:"phase"`
	IsGameOver bool                    `json:"is_over"`
	Summoned   bool                    `json:"has_summoned"`
}

type GameStartPayload struct {
	Type      string     `json:"type"`
	RoomID    string     `json:"room_id"`
	YourID    string     `json:"your_id"`
	GameState *GameState `json:"game_state"`
}

type GameStatePayload struct {
	Accept    bool       `json:"accepted"`
	Type      string     `json:"type"`
	GameState *GameState `json:"game_state"`
	Turn      bool       `json:"your_turn"`
	Error     string     `json:"error"`
}

type GameEvent struct {
	Type     GameEventType `json:"type"`
	Sequence int           `json:"sequence"`
	Payload  interface{}   `json:"payload"`
}

type CardMovedPayload struct {
	CardID   string `json:"card_id"`
	FromZone string `json:"from_zone"`
	ToZone   string `json:"to_zone"`
	Index    int    `json:"index"`
}

type StatChangedPayload struct {
	CardID   string `json:"card_id"`
	Stat     string `json:"stat"`
	NewValue int    `json:"new_value"`
}

type GameEndPayload struct {
	Winner        bool   `json:"didWin"`
	WinningReason string `json:"reason"`
	RedirectURL   string `json:"redirect_url"`
}

type PlayerAction struct {
	Type   string `json:"type"`
	CardID int    `json:"card_id"`
	Target int    `json:"card_target"`
}

func (app *application) NewGame(p1ID, p2ID string) *GameState {
	game := &GameState{
		Turn:       1,
		Players:    make(map[string]*PlayerState),
		ActiveTurn: p1ID,
		Phase:      "main",
	}

	game.Players[p1ID] = &PlayerState{ID: p1ID, Health: 20, Deck: app.generateDeck(0)}
	game.Players[p2ID] = &PlayerState{ID: p2ID, Health: 20, Deck: app.generateDeck(len(game.Players[p1ID].Deck))}

	game.Players[p1ID].DrawCards(4)
	game.Players[p2ID].DrawCards(4)

	return game
}

func StartGame(room *Room) error {
	for id, client := range room.Players {

		sanitizedState := room.Game.GetViewFor(id)
		payload := GameStatePayload{
			Type:      "main",
			GameState: &sanitizedState,
			Accept:    true,
		}

		if id == room.Game.ActiveTurn {
			payload.Turn = true
		} else {
			payload.Turn = false
		}

		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		select {
		case client.send <- jsonBytes:
		default:
			close(client.send)
			delete(room.Players, id)
		}
	}

	return nil
}
