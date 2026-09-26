package contracts

import (
	"encoding/json"
	"time"
)

// ---------------------------------------------------------------- item <-> asset

const (
	ItemVault        = "item.vault"         // move an inventory item into the NFT vault
	ItemUnvault      = "item.unvault"       // compensation: back to the character's bag
	ItemClaim        = "item.claim"         // move a vaulted item onto a character
	ItemSnapshot     = "item.snapshot"      // any item by id
	ItemGoldTransfer = "item.gold_transfer" // atomic gold payment between characters
	ItemAdminGrant   = "item.admin_grant"   // admin: grant gold/essence/items
)

type VaultReq struct {
	CharacterID string `json:"character_id,omitempty"`
	ItemID      string `json:"item_id"`
	OpID        string `json:"op_id"`
}

type GoldTransferReq struct {
	From   string `json:"from_character"`
	To     string `json:"to_character"`
	Amount int64  `json:"amount"`
	Fee    int64  `json:"fee"`
	OpID   string `json:"op_id"`
	Reason string `json:"reason"`
}

type GoldTransferResp struct {
	FromWallet Wallet `json:"from_wallet"`
}

type AdminGrantReq struct {
	CharacterID string `json:"character_id"`
	Gold        int64  `json:"gold"`
	Essence     int64  `json:"essence"`
	Base        string `json:"base,omitempty"`
	Rarity      string `json:"rarity,omitempty"`
	ItemLevel   int    `json:"item_level,omitempty"`
	Enhance     int    `json:"enhance,omitempty"`
	Reason      string `json:"reason"`
	OpID        string `json:"op_id"`
}

// ---------------------------------------------------------------- asset service

const (
	AssetMint      = "asset.mint"
	AssetClaim     = "asset.claim"
	AssetDeposit   = "asset.deposit_token" // re-vault a bound token's item
	AssetBurn      = "asset.burn"
	AssetTokens    = "asset.tokens"
	AssetToken     = "asset.token"
	AssetList      = "asset.list"
	AssetCancel    = "asset.cancel"
	AssetMarket    = "asset.market"
	AssetBuy       = "asset.buy"
	AssetWallet    = "asset.wallet"
	AssetWithdraw  = "asset.withdraw"
	AssetTonCredit = "asset.ton_credit" // from ton service: an incoming deposit
	AssetPolicyGet = "asset.policy.get"
	AssetPolicySet = "asset.policy.set"
	AssetStats     = "asset.stats"

	AssetWithdrawals      = "asset.withdrawals"       // admin: list by status
	AssetWithdrawalReview = "asset.withdrawal.review" // admin: approve / reject
	AssetWithdrawalResult = "asset.withdrawal.result" // ton service: sending / sent / failed
	AssetAdminCancel      = "asset.admin.cancel"      // admin: cancel a listing

	ChainBlocks = "chain.blocks"
	ChainBlock  = "chain.block"
	ChainProof  = "chain.proof"
	ChainInfo   = "chain.info"
)

// Token is an item tokenised on OMM Chain.
type Token struct {
	ID             int64           `json:"id"`
	ItemID         string          `json:"item_id"`
	Item           json.RawMessage `json:"item"` // OwnedItem snapshot
	ItemHash       string          `json:"item_hash"`
	Owner          string          `json:"owner_account"`
	State          string          `json:"state"` // pending | vault | claiming | bound | burned
	BoundCharacter string          `json:"bound_character,omitempty"`
	ListingID      int64           `json:"listing_id,omitempty"`
	Rarity         string          `json:"rarity"`
	Slot           string          `json:"slot"`
	MintedAt       time.Time       `json:"minted_at"`
}

type TokenReq struct {
	AccountID   string `json:"account_id"`
	CharacterID string `json:"character_id,omitempty"`
	TokenID     int64  `json:"token_id,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
}

type TokensResp struct {
	Tokens []Token `json:"tokens"`
}

type ChainTx struct {
	ID          int64           `json:"id"`
	Hash        string          `json:"hash"`
	Kind        string          `json:"kind"`
	TokenID     int64           `json:"token_id,omitempty"`
	From        string          `json:"from,omitempty"`
	To          string          `json:"to,omitempty"`
	Amount      int64           `json:"amount,omitempty"`
	Currency    string          `json:"currency,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	BlockHeight *int64          `json:"block_height,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type TokenView struct {
	Token      Token     `json:"token"`
	Provenance []ChainTx `json:"provenance"`
}

type ListReq struct {
	AccountID   string `json:"account_id"`
	CharacterID string `json:"character_id"`
	TokenID     int64  `json:"token_id"`
	Currency    string `json:"currency"` // TON | GOLD
	Price       int64  `json:"price"`    // nanoTON or gold
	RequestID   string `json:"request_id"`
}

type Listing struct {
	ID        int64     `json:"id"`
	TokenID   int64     `json:"token_id"`
	Seller    string    `json:"seller_account"`
	Currency  string    `json:"currency"`
	Price     int64     `json:"price"`
	Status    string    `json:"status"`
	Buyer     string    `json:"buyer_account,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Token     *Token    `json:"token,omitempty"`
	Mine      bool      `json:"mine,omitempty"`
}

type MarketReq struct {
	AccountID string `json:"account_id"`
	Currency  string `json:"currency,omitempty"`
	Slot      string `json:"slot,omitempty"`
	Rarity    string `json:"rarity,omitempty"`
	Sort      string `json:"sort,omitempty"` // new | price_asc | price_desc
	Mine      bool   `json:"mine,omitempty"`
	Status    string `json:"status,omitempty"` // admin: any status
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

type MarketResp struct {
	Listings []Listing `json:"listings"`
	Total    int       `json:"total"`
}

type BuyReq struct {
	AccountID   string `json:"account_id"`
	CharacterID string `json:"character_id"`
	ListingID   int64  `json:"listing_id"`
	RequestID   string `json:"request_id"`
}

type TonLedgerEntry struct {
	Delta     int64     `json:"delta"`
	Balance   int64     `json:"balance_after"`
	Reason    string    `json:"reason"`
	Ref       string    `json:"ref,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Withdrawal struct {
	ID        int64     `json:"id"`
	Account   string    `json:"account_id"`
	To        string    `json:"to_address"`
	Amount    int64     `json:"amount"`
	Fee       int64     `json:"fee"`
	Status    string    `json:"status"`
	TxHash    string    `json:"tx_hash,omitempty"`
	Reviewer  string    `json:"reviewer,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TonWallet struct {
	Balance        int64            `json:"balance"`
	Locked         int64            `json:"locked"`
	DepositAddress string           `json:"deposit_address"`
	DepositMemo    string           `json:"deposit_memo"`
	Network        string           `json:"network"`
	Ledger         []TonLedgerEntry `json:"ledger"`
	Withdrawals    []Withdrawal     `json:"withdrawals"`
	Policy         MarketPolicy     `json:"policy"`
}

type WithdrawReq struct {
	AccountID string `json:"account_id"`
	To        string `json:"to_address"`
	Amount    int64  `json:"amount"`
	RequestID string `json:"request_id"`
}

type TonCreditReq struct {
	TxHash string `json:"tx_hash"`
	LT     uint64 `json:"lt"`
	Amount int64  `json:"amount"`
	Memo   string `json:"memo"`
	Sender string `json:"sender"`
}

type WithdrawalsReq struct {
	Status string `json:"status"`
	Limit  int    `json:"limit"`
}

type WithdrawalsResp struct {
	Withdrawals []Withdrawal `json:"withdrawals"`
}

type WithdrawalReview struct {
	ID       int64  `json:"id"`
	Approve  bool   `json:"approve"`
	Reviewer string `json:"reviewer"`
	Note     string `json:"note"`
}

type WithdrawalResult struct {
	ID     int64  `json:"id"`
	Status string `json:"status"` // sending | sent | failed
	TxHash string `json:"tx_hash,omitempty"`
	Note   string `json:"note,omitempty"`
}

// MarketPolicy decides what may be tokenised and traded for what.
type MarketPolicy struct {
	MintMinRarity   string `json:"mint_min_rarity"`  // items below cannot become NFTs
	GoldMinRarity   string `json:"gold_min_rarity"`  // may be sold for gold
	TonMinRarity    string `json:"ton_min_rarity"`   // may be sold for TON (real money)
	FeeBps          int64  `json:"fee_bps"`          // marketplace fee, basis points
	MinPriceTon     int64  `json:"min_price_ton"`    // nanoTON
	MinPriceGold    int64  `json:"min_price_gold"`   // gold
	WithdrawMin     int64  `json:"withdraw_min"`     // nanoTON
	WithdrawFee     int64  `json:"withdraw_fee"`     // nanoTON network fee charged to the user
	AutoApproveMax  int64  `json:"auto_approve_max"` // nanoTON; 0 = every withdrawal needs an admin
	TradingEnabled  bool   `json:"trading_enabled"`  // kill switch
	WithdrawEnabled bool   `json:"withdraw_enabled"` // kill switch
}

type AssetStatsResp struct {
	Tokens        map[string]int `json:"tokens"` // by state
	ActiveTon     int            `json:"active_ton"`
	ActiveGold    int            `json:"active_gold"`
	Volume24hTon  int64          `json:"volume_24h_ton"`
	Volume24hGold int64          `json:"volume_24h_gold"`
	Sales24h      int            `json:"sales_24h"`
	FeesTon       int64          `json:"fees_ton"`
	UserBalances  int64          `json:"user_balances"`
	LockedTotal   int64          `json:"locked_total"`
	PendingWd     int            `json:"pending_withdrawals"`
	ChainHeight   int64          `json:"chain_height"`
}

// ---------------------------------------------------------------- chain

type Block struct {
	Height     int64     `json:"height"`
	PrevHash   string    `json:"prev_hash"`
	MerkleRoot string    `json:"merkle_root"`
	Hash       string    `json:"hash"`
	TxCount    int       `json:"tx_count"`
	Signature  string    `json:"signature"`
	CreatedAt  time.Time `json:"created_at"`
	Txs        []ChainTx `json:"txs,omitempty"`
}

type BlocksReq struct {
	Before int64 `json:"before,omitempty"`
	Limit  int   `json:"limit,omitempty"`
	Height int64 `json:"height,omitempty"`
}

type BlocksResp struct {
	Blocks []Block `json:"blocks"`
}

type ProofStep struct {
	Hash string `json:"hash"`
	Left bool   `json:"left"` // sibling is on the left
}

type ProofReq struct {
	TxHash string `json:"tx_hash"`
}

type ProofResp struct {
	Tx        ChainTx     `json:"tx"`
	Block     Block       `json:"block"`
	Proof     []ProofStep `json:"proof"`
	PublicKey string      `json:"public_key"`
	Valid     bool        `json:"valid"`
}

type ChainInfoResp struct {
	Name      string `json:"name"`
	Height    int64  `json:"height"`
	PublicKey string `json:"public_key"`
	Pending   int    `json:"pending_txs"`
	Txs       int64  `json:"txs"`
}

// ---------------------------------------------------------------- ton service

const (
	TonInfo       = "ton.info"
	TonMockCredit = "ton.mock.deposit" // dev only: simulate an incoming transfer
)

type TonInfoResp struct {
	Address string `json:"address"`
	Network string `json:"network"` // mainnet | testnet | mock
}

type MockDepositReq struct {
	Memo   string `json:"memo"`
	Amount int64  `json:"amount"`
	Sender string `json:"sender"`
}

// asset events
const (
	EvAssetMinted  = "game.asset.minted"
	EvAssetListed  = "game.asset.listed"
	EvAssetSold    = "game.asset.sold"
	EvTonDeposit   = "game.ton.deposit"
	EvTonWithdrawn = "game.ton.withdrawal"
)

type AssetEv struct {
	TokenID  int64  `json:"token_id"`
	Name     string `json:"name"`
	Rarity   string `json:"rarity"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Currency string `json:"currency,omitempty"`
	Price    int64  `json:"price,omitempty"`
}

type TonEv struct {
	Account string `json:"account_id"`
	Amount  int64  `json:"amount"`
	Ref     string `json:"ref"`
	Status  string `json:"status,omitempty"`
}
