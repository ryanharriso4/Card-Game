package main

type Room struct {
	Sequence uint64
	ID       string
	Players  map[string]*Client
	Game     *GameState
}
