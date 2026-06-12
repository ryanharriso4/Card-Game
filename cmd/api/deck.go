package main

import (
	"math/rand"

	"cardgame.ryanharris.net/internal/data"
)

func (app *application) generateDeck(startingUID int) []data.Card {

	deck, err := app.models.Cards.GetDeck()
	if err != nil {
		app.logger.Error(err.Error())
		return nil
	}

	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	for i := range deck {
		deck[i].ID = startingUID + i
	}

	return deck
}

func (gs *GameState) GetViewFor(viewingPlayerID string) GameState {

	view := GameState{
		Players:    make(map[string]*PlayerState),
		ActiveTurn: gs.ActiveTurn,
		Phase:      gs.Phase,
		IsGameOver: gs.IsGameOver,
	}

	for id, player := range gs.Players {
		pView := &PlayerState{
			ID:        player.ID,
			Health:    player.Health,
			HandSize:  len(player.Hand),
			DeckSize:  len(player.Deck),
			Graveyard: player.Graveyard,
			Board:     player.Board,
		}

		if id == viewingPlayerID {
			pView.Hand = player.Hand
			view.Players["you"] = pView
		} else {
			pView.Hand = nil
			view.Players["opponent"] = pView
		}

	}

	return view
}
