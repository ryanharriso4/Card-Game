package main

import (
	"fmt"
	"slices"

	"cardgame.ryanharris.net/internal/data"
)

func burn(players map[string]*PlayerState, action *PlayerAction, factor int, events *[]*GameEvent) bool {
	index := -1
	var foundCard data.Card
	var foundPlayer *PlayerState
	for _, player := range players {
		fmt.Printf("%v\ntarget:%d\n", player.Board, action.Target)
		idx, card := findCard(player.Board, action.Target)
		if idx != -1 {
			foundPlayer = player
			foundCard = card
			index = idx
			break
		}
	}

	if index == -1 || foundCard.IsPermanent == false {
		fmt.Println("Made it here")
		return false
	}

	if foundCard.Def-factor <= 0 {
		foundPlayer.Board = slices.Delete(foundPlayer.Board, index, index+1)
		foundPlayer.Graveyard = append(foundPlayer.Graveyard, foundCard)
		*events = append(*events, &GameEvent{PlayerID: foundPlayer.ID, Type: EventCardMoved, Payload: CardMovedPayload{Card: foundCard, FromZone: string(ZoneBoard), ToZone: string(ZoneGrave)}})
	}

	return true
}

func (p *PlayerState) DrawCards(n int) bool {
	if len(p.Deck) < n {
		return true
	}

	p.Hand = append(p.Hand, p.Deck[:n]...)
	p.Deck = p.Deck[n:]
	return false
}

func (p *PlayerState) DrawCardsEffect(n int, events *[]*GameEvent) bool {
	if len(p.Deck) < n {
		return true
	}

	for _, card := range p.Deck[:n] {
		*events = append(*events, &GameEvent{Type: EventCardDrawn, Payload: CardDrawnPayload{Card: card}})
	}

	p.Hand = append(p.Hand, p.Deck[:n]...)
	p.Deck = p.Deck[n:]
	return false
}

func destroy(players map[string]*PlayerState, action *PlayerAction, events *[]*GameEvent) bool {

	var index int
	var foundCard data.Card
	var foundPlayer *PlayerState
	for _, player := range players {
		fmt.Printf("%d\n", action.Target)
		idx, card := findCard(player.Board, action.Target)
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
	*events = append(*events, &GameEvent{PlayerID: foundPlayer.ID, Type: EventCardMoved, Payload: CardMovedPayload{Card: foundCard, FromZone: string(ZoneBoard), ToZone: string(ZoneGrave)}})

	return true
}

// Attacking logic returns if card can attack and if the other card can depend in that order. Ex. True, False means good attacker bad defender
func attack(AttackingPlayer *PlayerState, DefendingPlayer *PlayerState, attacking int, defending int) *[]*GameEvent {

	var events []*GameEvent

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

	if defendingIndex == -1 {
		return &[]*GameEvent{
			{
				Type: EventInvalidAttack,
				Payload: InvalidActionPayload{
					CardID: defending,
					Reason: string(InvalidTarget),
				},
			},
		}

	}

	if attackingIndex == -1 {
		return &[]*GameEvent{
			{
				Type: EventInvalidAttack,
				Payload: InvalidActionPayload{
					CardID: attacking,
					Reason: string(InvalidCardNotAvailable),
				},
			},
		}
	}

	if attackingCard.HasAttacked {
		return &[]*GameEvent{
			{
				Type: EventInvalidAttack,
				Payload: InvalidActionPayload{
					CardID: attacking,
					Reason: string(InvalidAlreadyAttacked),
				},
			},
		}
	}

	attackingCard.HasAttacked = true

	leftOverAttacking := attackingCard.Def - defendingCard.Atk
	leftOverDefending := defendingCard.Def - attackingCard.Atk

	if leftOverAttacking <= 0 {
		AttackingPlayer.Board = slices.Delete(AttackingPlayer.Board, attackingIndex, attackingIndex+1)
		AttackingPlayer.Graveyard = append(AttackingPlayer.Graveyard, attackingCard)
		events = append(events, &GameEvent{PlayerID: AttackingPlayer.ID, Type: EventCardMoved, Payload: CardMovedPayload{Card: attackingCard, FromZone: string(ZoneBoard), ToZone: string(ZoneGrave)}})
	}

	if leftOverDefending <= 0 {
		DefendingPlayer.Board = slices.Delete(DefendingPlayer.Board, defendingIndex, defendingIndex+1)
		DefendingPlayer.Graveyard = append(DefendingPlayer.Graveyard, defendingCard)
		events = append(events, &GameEvent{PlayerID: DefendingPlayer.ID, Type: EventCardMoved, Payload: CardMovedPayload{Card: defendingCard, FromZone: string(ZoneBoard), ToZone: string(ZoneGrave)}})
	}

	return &events

}

func findCard(list []data.Card, cardID int) (int, data.Card) {
	for i, c := range list {
		if c.ID == cardID {
			return i, c
		}
	}
	return -1, data.Card{}
}

func playCard(room *Room, player *PlayerState, action *PlayerAction) *[]*GameEvent {

	var events []*GameEvent

	index, card := findCard(player.Hand, action.CardID)
	if index == -1 {
		return &[]*GameEvent{{Type: EventInvalidAction,
			Payload: InvalidActionPayload{
				CardID: action.CardID,
				Reason: string(InvalidCardNotAvailable),
			}}}
	}

	if room.Game.ActiveTurn != player.ID {
		return &[]*GameEvent{{Type: EventInvalidAction,
			Payload: InvalidActionPayload{
				CardID: action.CardID,
				Reason: string(InvalidWrongTurn),
			}}}
	}

	if card.Type == "Creature" && room.Game.Summoned {
		return &[]*GameEvent{{Type: EventInvalidAction,
			Payload: InvalidActionPayload{
				CardID: action.CardID,
				Reason: string(InvalidAlreadySummoned),
			}}}
	}

	keywords := card.Keywords
	for _, keyword := range keywords {
		switch keyword.Name {
		case "burn":
			if !burn(room.Game.Players, action, keyword.Factor, &events) {
				return &[]*GameEvent{{Type: EventInvalidAction,
					Payload: InvalidActionPayload{
						CardID: action.CardID,
						Reason: string(InvalidTarget),
					}}}
			}
		case "draw":
			if player.DrawCardsEffect(keyword.Factor, &events) {
				return &[]*GameEvent{{Type: EventGameOver,
					Payload: GameEndPayload{Reason: string(GameEndDeckout)}}}
			}
		case "destroy":
			if !destroy(room.Game.Players, action, &events) {
				return &[]*GameEvent{{Type: EventInvalidAction,
					Payload: InvalidActionPayload{
						CardID: action.CardID,
						Reason: string(InvalidTarget),
					}}}
			}
		}

	}

	if card.IsPermanent == true {
		player.Board = append(player.Board, card)
		events = append(events, &GameEvent{PlayerID: player.ID, Type: EventCardMoved, Payload: CardMovedPayload{Card: card, FromZone: string(ZoneHand), ToZone: string(ZoneBoard)}})
	} else {
		player.Graveyard = append(player.Graveyard, card)
		events = append(events, &GameEvent{PlayerID: player.ID, Type: EventCardMoved, Payload: CardMovedPayload{Card: card, FromZone: string(ZoneHand), ToZone: string(ZoneGrave)}})
	}

	if card.Type == "Creature" {
		room.Game.Summoned = true
	}

	player.Hand = slices.Delete(player.Hand, index, index+1)

	fmt.Printf("%v\n", events)

	return &events
}

func attackCard(room *Room, player *PlayerState, action *PlayerAction) *[]*GameEvent {
	if room.Game.ActiveTurn != player.ID {
		return &[]*GameEvent{
			{
				Type: GameEventType(InvalidWrongTurn),
				Payload: InvalidActionPayload{
					CardID: action.CardID,
					Reason: string(InvalidWrongTurn),
				},
			},
		}
	}

	if room.Game.Phase != string(PhaseCombat) {
		return &[]*GameEvent{
			{
				Type: GameEventType(InvalidWrongTurn),
				Payload: InvalidActionPayload{
					CardID: action.CardID,
					Reason: string(InvalidPhase),
				},
			},
		}
	}

	var defendingPlayer *PlayerState
	for id, p := range room.Game.Players {
		if id != player.ID {
			defendingPlayer = p
			break
		}
	}

	events := attack(player, defendingPlayer, action.CardID, action.Target)

	return events

}

func attackPlayer(room *Room, player *PlayerState, action *PlayerAction) *[]*GameEvent {

	var defendingPlayer *PlayerState
	for id, p := range room.Game.Players {
		if id != player.ID {
			defendingPlayer = p
			break
		}
	}

	if len(defendingPlayer.Board) > 0 {
		return &[]*GameEvent{{Type: EventInvalidAttack, Payload: InvalidAttackPayload{CardID: -1, Reason: string(InvalidHasDefends)}}}
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
		return &[]*GameEvent{{Type: EventInvalidAttack, Payload: InvalidAttackPayload{CardID: action.CardID, Reason: string(InvalidCardNotAvailable)}}}
	}

	if attackCard.HasAttacked {
		return &[]*GameEvent{{Type: EventInvalidAttack, Payload: InvalidActionPayload{CardID: action.CardID, Reason: string(InvalidAlreadyAttacked)}}}
	}

	attackCard.HasAttacked = true

	defendingPlayer.Health -= attackCard.Atk

	if defendingPlayer.Health <= 0 {
		return &[]*GameEvent{{Type: EventGameOver, Payload: GameEndPayload{Reason: string(GameEndDamage)}}}
	}

	return &[]*GameEvent{{Type: EventStateChange, Payload: StatChangedPayload{CardID: -1, Stat: "health", NewValue: defendingPlayer.Health}}}
}

func HasPlayableCards(player *PlayerState) bool {
	for _, card := range player.Hand {
		if card.Speed == data.FAST {
			return true
		}
	}

	return false
}
