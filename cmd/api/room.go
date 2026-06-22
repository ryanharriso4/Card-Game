package main

type Room struct {
	Sequence int
	ID       string
	Players  map[string]*Client
	Game     *GameState
}
