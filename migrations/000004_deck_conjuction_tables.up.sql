CREATE TABLE card_keywords (
    card_id INT REFERENCES cards(id) ON DELETE CASCADE,
    keyword_id INT REFERENCES keywords(id) ON DELETE CASCADE,
    factor INT NOT NULL DEFAULT 1 CHECK (factor > 0),
    PRIMARY KEY (card_id, keyword_id)
);

CREATE TABLE deck_cards (
    deck_id INT REFERENCES decks(id) ON DELETE CASCADE,
    card_id INT REFERENCES cards(id) ON DELETE CASCADE,
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0), 
    PRIMARY KEY (deck_id, card_id)
);

CREATE INDEX idx_card_keywords_keyword ON card_keywords(keyword_id);
CREATE INDEX idx_deck_cards_card ON deck_cards(card_id);