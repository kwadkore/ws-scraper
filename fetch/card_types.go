package fetch

// Side identifies whether a card or title group belongs to the Weiss or
// Schwarz side.
type Side string

const (
	SideWeiss   Side = "W"
	SideSchwarz Side = "S"
)

// CardType identifies the kind of card.
type CardType string

const (
	CardTypeCharacter CardType = "CH"
	CardTypeEvent     CardType = "EV"
	CardTypeClimax    CardType = "CX"
)

// CardColor identifies a card's printed color.
type CardColor string

const (
	CardColorBlue   CardColor = "BLUE"
	CardColorGreen  CardColor = "GREEN"
	CardColorRed    CardColor = "RED"
	CardColorYellow CardColor = "YELLOW"
	CardColorPurple CardColor = "PURPLE"
)
