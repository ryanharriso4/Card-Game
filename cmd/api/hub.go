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
				app.logger.Info("New room started", "id", curRoomID)
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
			event := readJSON(room, msg.Payload, &action)
			if event != nil {
				app.broadcast(*event, room)
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

func readJSON(room *Room, payload []byte, action *PlayerAction) *[]*GameEvent {
	var syntaxError *json.SyntaxError
	var invalid *json.InvalidUnmarshalError
	var unmarshalTypeError *json.UnmarshalTypeError

	err := json.Unmarshal(payload, &action)
	if errors.As(err, &syntaxError) {
		newSeq := atomic.AddUint64(&room.Sequence, 1)
		line, col, contextSnippet := getJSONErrorContext(payload, syntaxError.Offset)
		detailedReason := fmt.Sprintf("Syntax error at line %d, col %d: %s\nContext: %s",
			line, col, syntaxError.Error(), contextSnippet)
		return &[]*GameEvent{{Sequence: int(newSeq), Type: EventInvalidRequest, Payload: InvalidRequestPayload{Type: "Syntax Error", Reason: detailedReason}}}
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		newSeq := atomic.AddUint64(&room.Sequence, 1)
		return &[]*GameEvent{{Sequence: int(newSeq), Type: EventInvalidRequest, Payload: InvalidRequestPayload{Type: "Unexpected EOF", Reason: "JSON ended unexpectedly"}}}
	}

	if errors.As(err, &unmarshalTypeError) {
		newSeq := atomic.AddUint64(&room.Sequence, 1)
		detailedReason := fmt.Sprintf("Expected %s but got value at offset %d", unmarshalTypeError.Type.String(), unmarshalTypeError.Offset)
		return &[]*GameEvent{{Sequence: int(newSeq), Type: EventInvalidRequest, Payload: InvalidRequestPayload{Type: "Type Error", Reason: detailedReason}}}
	}

	if errors.As(err, &invalid) {
		panic(err)
	}

	switch action.Type {
	case string(ActionAttack), string(ActionNextPhase), string(ActionPlay):
		return nil
	default:
		newSeq := atomic.AddUint64(&room.Sequence, 1)
		return &[]*GameEvent{{Sequence: int(newSeq), Type: EventInvalidRequest, Payload: InvalidRequestPayload{Type: "Unexpected Response", Reason: fmt.Sprintf("Unknown Response Type %s", action.Type)}}}

	}
}

func getJSONErrorContext(payload []byte, offset int64) (int, int, string) {
	if offset < 0 || int(offset) > len(payload) {
		return 1, 1, ""
	}

	line := 1
	col := 1

	// Calculate line and column numbers
	for i := 0; i < int(offset); i++ {
		if payload[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	// Extract a snippet around the error (up to 20 characters before and after)
	start := int(offset) - 20
	if start < 0 {
		start = 0
	}

	end := int(offset) + 20
	if end > len(payload) {
		end = len(payload)
	}

	// Highlight the exact error character using a carat indicator arrow
	snippet := string(payload[start:end])
	return line, col, snippet
}
