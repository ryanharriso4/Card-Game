package main

import (
	"fmt"
	"slices"

	"cardgame.ryanharris.net/internal/data"
)

func burn(players map[string]*PlayerState, factor int, target int) bool {
	var index int
	var foundCard data.Card
	var foundPlayer *PlayerState
	for _, player := range players {
		fmt.Printf("%d\n", target)
		idx, card := findCard(player.Board, target)
		if index != -1 {
			foundPlayer = player
			foundCard = card
			index = idx
			break
		}
	}

	fmt.Printf("%d\n%v", index, foundPlayer)

	if index == -1 || foundCard.IsPermanent == false {
		return false
	}

	if foundCard.Def-factor <= 0 {
		foundPlayer.Board = slices.Delete(foundPlayer.Board, index, index+1)
		foundPlayer.Graveyard = append(foundPlayer.Graveyard, foundCard)
	}

	return true
}

func (p *PlayerState) DrawCards(n int) ReasonLoss {
	if len(p.Deck) < n {
		return DECKEDOUT
	}
	p.Hand = append(p.Hand, p.Deck[:n]...)
	p.Deck = p.Deck[n:]
	return NOLOSS
}

func destroy(players map[string]*PlayerState, target int) bool {

	var index int
	var foundCard data.Card
	var foundPlayer *PlayerState
	for _, player := range players {
		fmt.Printf("%d\n", target)
		idx, card := findCard(player.Board, target)
		if index != -1 {
			foundPlayer = player
			foundCard = card
			index = idx
			break
		}
	}

	if index == -1 || foundCard.IsPermanent == false {
		return false
	}

	foundPlayer.Board = slices.Delete(foundPlayer.Board, index, index+1)
	foundPlayer.Graveyard = append(foundPlayer.Graveyard, foundCard)

	return true
}

// Attacking logic returns if card can attack and if the other card can depend in that order. Ex. True, False means good attacker bad defender
func attack(AttackingPlayer *PlayerState, DefendingPlayer *PlayerState, attacking int, defending int) (bool, bool) {

	fmt.Printf("%+v", AttackingPlayer.Board)
	fmt.Printf("%+v", DefendingPlayer.Board)

	attackingIndex := -1
	defendingIndex := -1
	attackingCard := data.Card{}
	defendingCard := data.Card{}
	for index, card := range AttackingPlayer.Board {
		if card.ID == attacking {
			attackingCard = card
			attackingIndex = index
			break
		}
	}

	for index, card := range DefendingPlayer.Board {
		if card.ID == defending {
			defendingCard = card
			defendingIndex = index
			break
		}
	}

	if defendingIndex == -1 || attackingIndex == -1 {
		return (attackingIndex != -1), (defendingIndex != -1)
	}

	leftOverAttacking := attackingCard.Def - defendingCard.Atk
	leftOverDefending := defendingCard.Def - attackingCard.Atk

	if leftOverAttacking <= 0 {
		AttackingPlayer.Board = slices.Delete(AttackingPlayer.Board, attackingIndex, attackingIndex+1)
		AttackingPlayer.Graveyard = append(AttackingPlayer.Graveyard, attackingCard)
	}

	if leftOverDefending <= 0 {
		DefendingPlayer.Board = slices.Delete(DefendingPlayer.Board, defendingIndex, defendingIndex+1)
		DefendingPlayer.Graveyard = append(DefendingPlayer.Graveyard, defendingCard)
	}

	return true, true

}

func findCard(list []data.Card, cardID int) (int, data.Card) {
	for i, c := range list {
		if c.ID == cardID {
			return i, c
		}
	}
	return -1, data.Card{}
}

func playCard(room *Room, player *PlayerState, action *PlayerAction) ([]GameEvent, error) {

	index, card := findCard(player.Hand, action.CardID)
	if index == -1 {
		return NOLOSS, false, "Card is not in hand"
	}

	if room.Game.ActiveTurn != player.ID {
		return NOLOSS, false, "It is not your turn"
	}

	if card.Type == "Creature" && room.Game.Summoned {
		return NOLOSS, false, "Already summoned this turn"
	}

	keywords := card.Keywords
	for _, keyword := range keywords {
		switch keyword.Name {
		case "burn":
			if !burn(room.Game.Players, keyword.Factor, action.Target) {
				return NOLOSS, false, "invalid target"
			}
		case "draw":
			if player.DrawCards(keyword.Factor) != NOLOSS {
				return DECKEDOUT, true, string(OpponentDeckOut)
			}
		case "destroy":
			if !destroy(room.Game.Players, action.Target) {
				return NOLOSS, false, "invalid target"
			}
		}

	}

	if card.IsPermanent == true {
		player.Board = append(player.Board, card)
	} else {
		player.Graveyard = append(player.Graveyard, card)
	}

	if card.Type == "Creature" {
		room.Game.Summoned = true
	}

	player.Hand = slices.Delete(player.Hand, index, index+1)

	return NOLOSS, true, "na"
}

func attackCard(room *Room, player *PlayerState, action *PlayerAction) (bool, string) {
	if room.Game.ActiveTurn != player.ID {
		return false, "invalid: It is not your turn"
	}

	if room.Game.Phase != string(PhaseCombat) {
		return false, "invalid: It is not combat"
	}

	var defendingPlayer *PlayerState
	for id, p := range room.Game.Players {
		if id != player.ID {
			defendingPlayer = p
			break
		}
	}

	attacking, defending := attack(player, defendingPlayer, action.CardID, action.Target)

	switch {
	case !attacking && !defending:
		return false, "invalid: invalid attacker and invalid defender"
	case !attacking:
		return false, "invalid: invalid attacker"
	case !defending:
		return false, "invalid: invalid defender"

	default:
		return true, "na"

	}

}

func attackPlayer(room *Room, player *PlayerState, action *PlayerAction) (ReasonLoss, bool, string) {

	var defendingPlayer *PlayerState
	for id, p := range room.Game.Players {
		if id != player.ID {
			defendingPlayer = p
			break
		}
	}

	if len(defendingPlayer.Board) > 0 {
		return NOLOSS, false, "Opponent has creatures"
	}

	var attackCard data.Card
	attackingIndex := -1
	for index, card := range player.Board {
		if card.ID == action.CardID {
			attackCard = card
			attackingIndex = index
		}
	}

	if attackingIndex == -1 {
		return NOLOSS, false, "Attacking card not found"
	}

	defendingPlayer.Health -= attackCard.Atk

	if defendingPlayer.Health <= 0 {
		return DAMAGE, true, "na"
	}

	return NOLOSS, true, "na"
}

func HasPlayableCards(player *PlayerState) bool {
	for _, card := range player.Hand {
		if card.Speed == data.FAST {
			return true
		}
	}

	return false
}
