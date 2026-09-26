// Package contracts is the single source of truth for inter-service NATS
// subjects, request/response payloads and domain event payloads.
package contracts

import (
	"encoding/json"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/auth"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/bestiary"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/combat"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/dungeon"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/items"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/progression"
	"github.com/mrjvadi/ommrpg/backend/pkg/game/world"
)

// ---------------------------------------------------------------- identity

const (
	IdentityTelegramLogin = "identity.telegram.login"
	IdentityDevLogin      = "identity.dev.login"
	IdentityGet           = "identity.get"
	IdentitySettingsGet   = "identity.settings.get"
	IdentitySettingsPut   = "identity.settings.put"
)

type Account struct {
	ID          string    `json:"id"`
	TelegramID  int64     `json:"telegram_id,omitempty"`
	Username    string    `json:"username,omitempty"`
	DisplayName string    `json:"display_name"`
	Language    string    `json:"language,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type TelegramLoginReq struct {
	User auth.TelegramUser `json:"user"`
}

type DevLoginReq struct {
	Username string `json:"username"`
}

type AccountReq struct {
	AccountID string `json:"account_id"`
}

type SettingsGetReq struct {
	AccountID string `json:"account_id"`
	Key       string `json:"key"`
}

type SettingsPutReq struct {
	AccountID string          `json:"account_id"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
}

type SettingsValue struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// ---------------------------------------------------------------- sprite

const (
	SpriteRandom   = "sprite.random"
	SpriteValidate = "sprite.validate"
)

// Layer is one part of an LPC-style character recipe.
type Layer struct {
	Item    string   `json:"item"`
	Variant string   `json:"variant,omitempty"`
	Colors  []string `json:"colors,omitempty"`
}

// Recipe describes a character sprite sheet.
type Recipe struct {
	BodyType string  `json:"body_type"`
	Layers   []Layer `json:"layers"`
}

type SpriteRandomReq struct {
	Seed     uint64 `json:"seed,string"`
	BodyType string `json:"body_type,omitempty"`
}

type SpriteValidateResp struct {
	Recipe  Recipe   `json:"recipe"`
	Dropped []string `json:"dropped,omitempty"`
}

// ---------------------------------------------------------------- character

const (
	CharacterCreate        = "character.create"
	CharacterList          = "character.list"
	CharacterGet           = "character.get"
	CharacterPublic        = "character.public"
	CharacterAllocate      = "character.allocate"
	CharacterCombatProfile = "character.combat_profile"
	CharacterLocationGet   = "character.location.get"
	CharacterLocationSet   = "character.location.set"
)

type CreateCharacterReq struct {
	AccountID  string  `json:"account_id"`
	Name       string  `json:"name"`
	Appearance *Recipe `json:"appearance,omitempty"`
	BodyType   string  `json:"body_type,omitempty"`
}

type Character struct {
	ID         string                 `json:"id"`
	AccountID  string                 `json:"account_id"`
	Name       string                 `json:"name"`
	Level      int                    `json:"level"`
	XP         int64                  `json:"xp"`
	XPToNext   int64                  `json:"xp_to_next"`
	Attributes progression.Attributes `json:"attributes"`
	FreePoints int                    `json:"free_points"`
	Class      string                 `json:"class,omitempty"`
	WorldID    int                    `json:"world_id"`
	Appearance Recipe                 `json:"appearance"`
	Derived    *progression.Derived   `json:"derived,omitempty"`
	FX         *items.FX              `json:"fx,omitempty"`
	// RootHint is the only visible trace of the hidden Root: a vague omen.
	RootHint  string    `json:"root_hint,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CharacterReq struct {
	CharacterID string `json:"character_id"`
	// AccountID, when set, enforces ownership.
	AccountID string `json:"account_id,omitempty"`
}

type CharacterListResp struct {
	Characters []Character `json:"characters"`
}

// PublicCharacter is what other players see.
type PublicCharacter struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Level      int       `json:"level"`
	Class      string    `json:"class,omitempty"`
	Appearance Recipe    `json:"appearance"`
	FX         *items.FX `json:"fx,omitempty"`
}

type AllocateReq struct {
	CharacterID string                 `json:"character_id"`
	Points      progression.Attributes `json:"points"`
}

// CombatProfile is everything combat needs about a character.
type CombatProfile struct {
	CharacterID string              `json:"character_id"`
	Level       int                 `json:"level"`
	Derived     progression.Derived `json:"derived"`
	Hidden      progression.Hidden  `json:"hidden"`
	WorldID     int                 `json:"world_id"`
}

type Location struct {
	CharacterID string  `json:"character_id"`
	Zone        string  `json:"zone"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
}

// ---------------------------------------------------------------- world

const (
	WorldList    = "world.list"
	WorldGet     = "world.get"
	WorldDefault = "world.default"
	WorldChunk   = "world.chunk"
	WorldSpecies = "world.species"
)

type WorldInfo struct {
	world.Params
	SpawnX int `json:"spawn_x"`
	SpawnY int `json:"spawn_y"`
}

type WorldReq struct {
	WorldID int `json:"world_id"`
}

type WorldListResp struct {
	Worlds []WorldInfo `json:"worlds"`
}

type ChunkReq struct {
	WorldID int `json:"world_id"`
	CX      int `json:"cx"`
	CY      int `json:"cy"`
}

type Chunk struct {
	WorldID  int              `json:"world_id"`
	Terrain  world.Terrain    `json:"terrain"`
	Monsters []bestiary.Spawn `json:"monsters"`
	Entrance *world.Entrance  `json:"entrance,omitempty"`
}

type SpeciesResp struct {
	Species []bestiary.Species `json:"species"`
}

// ---------------------------------------------------------------- items

const (
	ItemList    = "item.list"
	ItemEquip   = "item.equip"
	ItemUnequip = "item.unequip"
	ItemEnhance = "item.enhance"
	ItemSalvage = "item.salvage"
	ItemBonuses = "item.bonuses"
)

type OwnedItem struct {
	ID          string             `json:"id"`
	Item        items.Item         `json:"item"`
	State       items.State        `json:"state"`
	DisplayName string             `json:"display_name"`
	Equipped    string             `json:"equipped,omitempty"` // slot when equipped
	Stats       map[string]float64 `json:"stats"`
	XPToNext    int64              `json:"xp_to_next"`
	// Next enhancement preview.
	EnhanceChance  float64   `json:"enhance_chance"`
	EnhanceGold    int64     `json:"enhance_gold"`
	EnhanceEssence int64     `json:"enhance_essence"`
	SalvageEssence int64     `json:"salvage_essence"`
	SalvageGold    int64     `json:"salvage_gold"`
	Source         string    `json:"source"`
	CreatedAt      time.Time `json:"created_at"`
}

type Wallet struct {
	Gold    int64 `json:"gold"`
	Essence int64 `json:"essence"`
}

type InventoryResp struct {
	Items  []OwnedItem `json:"items"`
	Wallet Wallet      `json:"wallet"`
}

type ItemActionReq struct {
	CharacterID string `json:"character_id"`
	ItemID      string `json:"item_id,omitempty"`
	Slot        string `json:"slot,omitempty"`
	// RequestID makes enhance/salvage idempotent across client retries.
	RequestID string `json:"request_id,omitempty"`
}

type EnhanceResp struct {
	Result items.EnhanceResult `json:"result"`
	Item   OwnedItem           `json:"item"`
	Wallet Wallet              `json:"wallet"`
}

type SalvageResp struct {
	Essence int64  `json:"essence"`
	Gold    int64  `json:"gold"`
	Wallet  Wallet `json:"wallet"`
}

type BonusesResp struct {
	Bonuses progression.Bonuses `json:"bonuses"`
	Looks   []Layer             `json:"looks"`
	// FX of the equipped weapon (element glow / enhancement aura).
	FX *items.FX `json:"fx,omitempty"`
}

// ---------------------------------------------------------------- presence

const (
	PresenceEnter    = "presence.enter"
	PresenceMove     = "presence.move"
	PresenceGet      = "presence.get"
	PresenceNearby   = "presence.nearby"
	PresenceTeleport = "presence.teleport"
	// PresenceLook tells nearby players to refresh someone's appearance.
	PresenceLook = "presence.look"
	// PresenceStatsSubject reports online players per zone (admin).
	PresenceStatsSubject = "presence.stats"
)

type PresenceStats struct {
	Online int            `json:"online"`
	Zones  map[string]int `json:"zones"`
}

type Position struct {
	CharacterID string  `json:"character_id"`
	Zone        string  `json:"zone"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Dir         string  `json:"dir,omitempty"`
	Anim        string  `json:"anim,omitempty"`
	At          int64   `json:"at"` // unix ms
}

type MoveReq struct {
	CharacterID string  `json:"character_id"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Dir         string  `json:"dir"`
	Anim        string  `json:"anim"`
	Seq         int64   `json:"seq"`
}

type MoveResp struct {
	Accepted bool     `json:"accepted"`
	Position Position `json:"position"`
}

type TeleportReq struct {
	CharacterID string  `json:"character_id"`
	Zone        string  `json:"zone"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Reason      string  `json:"reason"`
}

type NearbyResp struct {
	Players []Position `json:"players"`
}

// ---------------------------------------------------------------- combat

const (
	CombatAttack   = "combat.attack"
	CombatMonsters = "combat.monsters"
	CombatPlayer   = "combat.player"
)

type AttackReq struct {
	CharacterID string `json:"character_id"`
	TargetID    string `json:"target_id"`
}

type AttackResp struct {
	Hit       combat.Hit  `json:"hit"`
	TargetHP  int         `json:"target_hp"`
	TargetMax int         `json:"target_max"`
	Killed    bool        `json:"killed"`
	XP        int64       `json:"xp,omitempty"`
	Loot      *items.Loot `json:"loot,omitempty"`
	Counter   *combat.Hit `json:"counter,omitempty"`
	PlayerHP  int         `json:"player_hp"`
	PlayerMax int         `json:"player_max"`
	Died      bool        `json:"died,omitempty"`
}

type MonstersReq struct {
	CharacterID string `json:"character_id"`
}

type MonsterState struct {
	ID        string         `json:"id"`
	Spawn     bestiary.Spawn `json:"spawn"`
	HP        int            `json:"hp"`
	MaxHP     int            `json:"max_hp"`
	DeadUntil int64          `json:"dead_until,omitempty"` // unix ms; 0 = alive, -1 = gone for good
}

type MonstersResp struct {
	Zone     string         `json:"zone"`
	Monsters []MonsterState `json:"monsters"`
}

type PlayerVitals struct {
	HP    int `json:"hp"`
	MaxHP int `json:"max_hp"`
}

// ---------------------------------------------------------------- dungeon

const (
	DungeonEnter     = "dungeon.enter"
	DungeonDescend   = "dungeon.descend"
	DungeonLeave     = "dungeon.leave"
	DungeonFloor     = "dungeon.floor"
	DungeonInstance  = "dungeon.instance"
	DungeonOpenChest = "dungeon.open_chest"
)

// Instance is a running dungeon run, stored in Dragonfly.
type Instance struct {
	ID         string       `json:"id"`
	Spec       dungeon.Spec `json:"spec"`
	EntranceID string       `json:"entrance_id"`
	WorldID    int          `json:"world_id"`
	OwnerID    string       `json:"owner_id"`
	ReturnX    float64      `json:"return_x"`
	ReturnY    float64      `json:"return_y"`
	CreatedAt  time.Time    `json:"created_at"`
}

type DungeonEnterReq struct {
	CharacterID string `json:"character_id"`
	EntranceID  string `json:"entrance_id"`
}

type DungeonReq struct {
	CharacterID string `json:"character_id"`
	InstanceID  string `json:"instance_id,omitempty"`
	ChestID     string `json:"chest_id,omitempty"`
}

type DungeonView struct {
	InstanceID string         `json:"instance_id"`
	Name       string         `json:"name"`
	Tier       int            `json:"tier"`
	Zone       string         `json:"zone"`
	Floor      *dungeon.Floor `json:"floor"`
	Position   Position       `json:"position"`
}

type LeaveResp struct {
	Position Position `json:"position"`
}

type ChestResp struct {
	Loot items.Loot `json:"loot"`
}

// ---------------------------------------------------------------- history

const (
	HistoryRecent = "history.recent"
	HistoryFirsts = "history.firsts"
)

type HistoryReq struct {
	Limit int `json:"limit"`
}

type HistoryEntry struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Time    time.Time      `json:"time"`
	Summary string         `json:"summary"`
	Data    map[string]any `json:"data,omitempty"`
}

type HistoryResp struct {
	Entries []HistoryEntry `json:"entries"`
}

// ---------------------------------------------------------------- events

const (
	EvCharacterCreated  = "game.character.created"
	EvCharacterLeveled  = "game.character.leveled"
	EvCharacterAwakened = "game.character.awakened"
	EvCharacterDied     = "game.character.died"
	EvMonsterKilled     = "game.monster.killed"
	EvChestOpened       = "game.chest.opened"
	EvItemDropped       = "game.item.dropped"
	EvItemEnhanced      = "game.item.enhanced"
	EvItemSalvaged      = "game.item.salvaged"
	EvItemLeveled       = "game.item.leveled"
	EvDungeonEntered    = "game.dungeon.entered"
	EvDungeonCleared    = "game.dungeon.cleared"
)

type CharacterCreatedEv struct {
	CharacterID string `json:"character_id"`
	AccountID   string `json:"account_id"`
	Name        string `json:"name"`
	WorldID     int    `json:"world_id"`
}

type CharacterLeveledEv struct {
	CharacterID string `json:"character_id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
}

type CharacterAwakenedEv struct {
	CharacterID string `json:"character_id"`
	Name        string `json:"name"`
	Class       string `json:"class"`
}

type CharacterDiedEv struct {
	CharacterID string `json:"character_id"`
	Zone        string `json:"zone"`
	KilledBy    string `json:"killed_by"`
}

type MonsterKilledEv struct {
	CharacterID string        `json:"character_id"`
	MonsterID   string        `json:"monster_id"`
	Species     int           `json:"species"`
	SpeciesName string        `json:"species_name"`
	Rank        bestiary.Rank `json:"rank"`
	Level       int           `json:"level"`
	Zone        string        `json:"zone"`
	XP          int64         `json:"xp"`
	WeaponKind  string        `json:"weapon_kind"`
	Loot        items.Loot    `json:"loot"`
	// DamageShare is this participant's fraction of the damage dealt.
	DamageShare float64 `json:"damage_share"`
}

type ChestOpenedEv struct {
	CharacterID string     `json:"character_id"`
	ChestID     string     `json:"chest_id"`
	Loot        items.Loot `json:"loot"`
}

type ItemDroppedEv struct {
	CharacterID string     `json:"character_id"`
	ItemID      string     `json:"item_id"`
	Item        items.Item `json:"item"`
	Source      string     `json:"source"`
}

type ItemEnhancedEv struct {
	CharacterID string              `json:"character_id"`
	ItemID      string              `json:"item_id"`
	ItemName    string              `json:"item_name"`
	Result      items.EnhanceResult `json:"result"`
}

type ItemSalvagedEv struct {
	CharacterID string `json:"character_id"`
	ItemID      string `json:"item_id"`
}

type ItemLeveledEv struct {
	CharacterID string `json:"character_id"`
	ItemID      string `json:"item_id"`
	ItemName    string `json:"item_name"`
	Level       int    `json:"level"`
}

type DungeonEnteredEv struct {
	CharacterID string `json:"character_id"`
	InstanceID  string `json:"instance_id"`
	EntranceID  string `json:"entrance_id"`
	Name        string `json:"name"`
	Tier        int    `json:"tier"`
}

type DungeonClearedEv struct {
	CharacterID string `json:"character_id"`
	InstanceID  string `json:"instance_id"`
	EntranceID  string `json:"entrance_id"`
	Name        string `json:"name"`
	Tier        int    `json:"tier"`
}
