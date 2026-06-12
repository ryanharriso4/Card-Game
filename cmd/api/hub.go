package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

			payload := GameStatePayload{}

			var action PlayerAction
			err := readJSON(msg.Payload, &action)
			if err != nil {
				payload.Accept = false
				payload.Error = err.Error()
				app.broadcastGS(room, payload)
				break
			}

			if action.Type == string(ActionPlay) {
				reason, accept, errorMsg := playCard(room, room.Game.Players[msg.Sender.ID], &action)
				if reason != NOLOSS {
					app.broadcastEnd(room, reason)
				} else {
					payload.Type = room.Game.Phase
					payload.Accept, payload.Error = accept, errorMsg
					app.broadcastGS(room, payload)
				}

			}

			if action.Type == string(ActionAttack) {
				var reason ReasonLoss
				var acceptable bool
				var errMessage string

				if action.Target == -1 {
					reason, acceptable, errMessage = attackPlayer(room, room.Game.Players[msg.Sender.ID], &action)
				} else {
					acceptable, errMessage = attackCard(room, room.Game.Players[msg.Sender.ID], &action)
				}

				if reason != NOLOSS {
					app.broadcastEnd(room, DAMAGE)
				} else {
					state := GameStatePayload{
						Accept:    acceptable,
						Type:      string(PhaseCombat),
						GameState: room.Game,
						Error:     errMessage,
					}

					app.broadcastGS(room, state)
				}
			}

			if action.Type == string(ActionNextPhase) {

				state := GameStatePayload{
					Accept:    true,
					Type:      string(EventChangePhase),
					GameState: room.Game,
					Error:     "na",
				}

				if room.Game.ActiveTurn != msg.Sender.ID {
					state.Accept = false
					state.Error = "It is not your turn"
					app.broadcastGS(room, state)
				} else {

					loss := NOLOSS
					current := room.Game.Phase
					var phase string
					switch current {
					case string(PhaseMain):
						phase = string(PhaseCombat)
					case string(PhaseCombat):
						phase = string(PhaseMain)
						room.Game.Turn += 1
						for id := range room.Players {
							if room.Game.ActiveTurn != id {
								room.Game.ActiveTurn = id
								loss = room.Game.Players[id].DrawCards(1)
								break
							}
						}
						state.Type = string(EventStateChange)
						room.Game.Summoned = false
					}

					room.Game.Phase = phase

					if loss != NOLOSS {
						app.broadcastEnd(room, loss)
					} else {
						app.broadcastGS(room, state)
					}
				}
			}

		}
	}
}

func (app *application) broadcastGS(room *Room, state GameStatePayload) {
	for id, client := range room.Players {

		if id == room.Game.ActiveTurn {
			state.Turn = true
		} else {
			state.Turn = false
		}

		sanitizedState := room.Game.GetViewFor(id)
		state.GameState = &sanitizedState

		jsonBytes, err := json.Marshal(state)
		if err != nil {
			app.logger.Error(err.Error())
		}

		select {
		case client.send <- jsonBytes:
		default:
			close(client.send)
			delete(room.Players, id)
		}
	}
}

func (app *application) broadcastEnd(room *Room, reason ReasonLoss) {

	state := GameEndPayload{
		RedirectURL: "/v1/healthcheck",
	}
	for id, client := range room.Players {
		switch reason {
		case DECKEDOUT:
			if room.Game.ActiveTurn == id {
				state.Winner = false
				state.WinningReason = string(PlayerDeckOut)
			} else {
				state.Winner = true
				state.WinningReason = string(OpponentDeckOut)
			}
		case DAMAGE:
			if room.Game.ActiveTurn == id {
				state.Winner = true
				state.WinningReason = string(OpponentDied)
			} else {
				state.Winner = false
				state.WinningReason = string(PlayerDied)
			}
		}

		jsonBytes, err := json.Marshal(state)
		if err != nil {
			app.logger.Error(err.Error())
		}

		select {
		case client.send <- jsonBytes:
		default:
			client.hub.unregister <- client
			close(client.send)
			delete(room.Players, id)
		}
	}

	for _, client := range room.Players {
		client.hub.unregister <- client
	}
}

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
