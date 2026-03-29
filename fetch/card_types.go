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

// Trigger identifies a climax trigger icon.
type Trigger string

const (
	TriggerSoul      Trigger = "SOUL"
	TriggerComeback  Trigger = "COMEBACK"
	TriggerDraw      Trigger = "DRAW"
	TriggerPool      Trigger = "POOL"
	TriggerTreasure  Trigger = "TREASURE"
	TriggerShot      Trigger = "SHOT"
	TriggerReturn    Trigger = "RETURN"
	TriggerGate      Trigger = "GATE"
	TriggerStandby   Trigger = "STANDBY"
	TriggerChoice    Trigger = "CHOICE"
	TriggerDiscovery Trigger = "DISCOVERY"
	TriggerChance    Trigger = "CHANCE"
)

// ExpansionSourceType identifies where the enriched expansion metadata came from.
type ExpansionSourceType string

const (
	ExpansionSourceTypeProductPage  ExpansionSourceType = "product_page"
	ExpansionSourceTypePromoListing ExpansionSourceType = "promo_listing"
)
