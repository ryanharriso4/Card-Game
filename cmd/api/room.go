package main

type Room struct {
	ID      string
	Players map[string]*Client
	Game    *GameState
}
