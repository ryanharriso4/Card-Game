package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
)

type Hub struct {
	rooms      map[string]*Room
	broadcast  chan GameMessage
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan GameMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]*Room),
	}
}

func (app *application) run() {
	curRoomID := "room_1"

	for {
		select {
		case client := <-app.hub.register:
			client.roomID = curRoomID

			if app.hub.rooms[curRoomID] == nil {
				app.hub.rooms[curRoomID] = &Room{ID: curRoomID, Players: make(map[string]*Client)}
			}

			room := app.hub.rooms[curRoomID]

			room.Players[client.ID] = client

			if len(room.Players) == 2 {

				var playerIDs []string
				for id := range room.Players {
					playerIDs = append(playerIDs, id)
				}

				room.Game = app.NewGame(playerIDs[0], playerIDs[1])
				err := StartGame(room)
				if err != nil {
					app.logger.Error(err.Error())
				}
				curRoomID = fmt.Sprintf("room_%d", len(app.hub.rooms)+1)

			}

		case client := <-app.hub.unregister:
			roomID := client.roomID
			room := app.hub.rooms[roomID]

			if room != nil {
				delete(room.Players, client.ID)
				close(client.send)

				if len(room.Players) == 0 {
					delete(app.hub.rooms, roomID)
					if roomID == curRoomID {
						curRoomID = fmt.Sprintf("room_%d", len(app.hub.rooms)-1)
					} else if len(app.hub.rooms) == 0 {
						curRoomID = "room_1"
					}
				}
			}

		case msg := <-app.hub.broadcast:
			room := app.hub.rooms[msg.RoomID]
			if room == nil || room.Game == nil {
				continue
			}

			var action PlayerAction
			err := readJSON(msg.Payload, &action)
			if err != nil {
				fmt.Println("error error error")
				break
			}

			var events = []*GameEvent{}

			if action.Type == string(ActionPlay) {
				events = *playCard(room, room.Game.Players[msg.Sender.ID], &action)
			}

			if action.Type == string(ActionAttack) {

				if action.Target == -1 {
					events = *attackPlayer(room, room.Game.Players[msg.Sender.ID], &action)
				} else {
					events = *attackCard(room, room.Game.Players[msg.Sender.ID], &action)
				}
			}

			if action.Type == string(ActionNextPhase) {
				fmt.Printf("Action type: %q\n", action.Type)

				if room.Game.ActiveTurn != msg.Sender.ID {
					events = append(events, &GameEvent{Type: EventInvalidAction, Payload: InvalidActionPayload{CardID: -1, Reason: string(InvalidWrongTurn)}})
				} else {

					current := room.Game.Phase
					var phase string
					switch current {
					case string(PhaseMain):
						newSeq := atomic.AddUint64(&room.Sequence, 1)
						phase = string(PhaseCombat)
						events = append(events, &GameEvent{Sequence: int(newSeq), Type: EventChangePhase, Payload: PhaseChangePayload{ChangeTurn: false, Phase: string(PhaseCombat)}})
						room.Game.HasAttacked = make(map[int]struct{})
					case string(PhaseCombat):
						phase = string(PhaseMain)
						room.Game.Turn += 1
						for id := range room.Players {
							if room.Game.ActiveTurn != id {
								room.Game.ActiveTurn = id
								room.Game.Players[id].DrawCardsEffect(room, 1, &events)
								break
							}
						}

						if len(events) != 0 {
							newSeq := atomic.AddUint64(&room.Sequence, 1)
							events = append(events, &GameEvent{Sequence: int(newSeq), Type: EventChangePhase, Payload: PhaseChangePayload{ChangeTurn: false, Phase: string(PhaseMain)}})
						}
						room.Game.Summoned = false
					}

					room.Game.Phase = phase

				}
			}

			app.broadcast(events, room)

		}
	}
}

func (app *application) broadcast(events []*GameEvent, room *Room) {
	fmt.Println("Broadcasting")
	for id, client := range room.Players {
		playerEvents := make([]GameEvent, len(events))

		for i, ev := range events {
			playerEvents[i] = *ev

			if client.ID == room.Game.ActiveTurn {

				playerEvents[i].Player = true
			} else {
				if ev.Type == EventCardDrawn {
					playerEvents[i].Payload = nil
				}
				playerEvents[i].Player = false
			}

			if ev.Type == EventCardMoved && client.ID != ev.PlayerID {
				playerEvents[i].Player = false
			}

			switch v := ev.Payload.(type) {
			case GameEndPayload:
				if v.Reason == string(GameEndDamage) && client.ID == room.Game.ActiveTurn {
					v.Winner = true
					playerEvents[i].Payload = v
				}

				if v.Reason == string(GameEndDeckout) && client.ID != room.Game.ActiveTurn {
					v.Winner = true
					playerEvents[i].Payload = v
				}
			}
		}

		jsonBytes, err := json.Marshal(playerEvents)
		if err != nil {
			app.logger.Error(err.Error())
			continue
		}

		select {
		case client.send <- jsonBytes:
		default:
			close(client.send)
			delete(room.Players, id)
		}
	}
}

// func (app *application) broadcastGS(room *Room, state GameStatePayload) {
// 	for id, client := range room.Players {

// 		if id == room.Game.ActiveTurn {
// 			state.Turn = true
// 		} else {
// 			state.Turn = false
// 		}

// 		if state.Type == string(EventStateChange) && state.Player == id {

// 		}

// 		sanitizedState := room.Game.GetViewFor(id)
// 		state.GameState = &sanitizedState

// 		jsonBytes, err := json.Marshal(state)
// 		if err != nil {
// 			app.logger.Error(err.Error())
// 		}

// 		select {
// 		case client.send <- jsonBytes:
// 		default:
// 			close(client.send)
// 			delete(room.Players, id)
// 		}
// 	}
// }

// func (app *application) broadcastEnd(room *Room, reason ReasonLoss) {

// 	state := GameEndPayload{
// 		RedirectURL: "/v1/healthcheck",
// 	}
// 	for id, client := range room.Players {
// 		switch reason {
// 		case DECKEDOUT:
// 			if room.Game.ActiveTurn == id {
// 				state.Winner = false
// 				state.WinningReason = string(PlayerDeckOut)
// 			} else {
// 				state.Winner = true
// 				state.WinningReason = string(OpponentDeckOut)
// 			}
// 		case DAMAGE:
// 			if room.Game.ActiveTurn == id {
// 				state.Winner = true
// 				state.WinningReason = string(OpponentDied)
// 			} else {
// 				state.Winner = false
// 				state.WinningReason = string(PlayerDied)s
// 			}
// 		}

// 		jsonBytes, err := json.Marshal(state)
// 		if err != nil {
// 			app.logger.Error(err.Error())
// 		}

// 		select {
// 		case client.send <- jsonBytes:
// 		default:
// 			client.hub.unregister <- client
// 			close(client.send)
// 			delete(room.Players, id)
// 		}
// 	}

// 	for _, client := range room.Players {
// 		client.hub.unregister <- client
// 	}
// }

func readJSON(payload []byte, action *PlayerAction) error {
	var syntaxError *json.SyntaxError
	var invalid *json.InvalidUnmarshalError

	err := json.Unmarshal(payload, &action)
	if errors.As(err, &syntaxError) {
		return fmt.Errorf("There is an error at %d", syntaxError.Offset)
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		return errors.New("JSON ended unexpectedly")
	}

	if errors.As(err, &invalid) {
		panic(err)
	}

	return nil
}
